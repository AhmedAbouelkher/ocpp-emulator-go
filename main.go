package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/v4"
	ocpp16 "github.com/lorenzodonini/ocpp-go/ocpp1.6"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/firmware"
	"github.com/lorenzodonini/ocpp-go/ws"
	log "github.com/sirupsen/logrus"
)

const (
	appVersion = "4.0.0"
)

var (
	csUrl, controlPort, dbPath string
	showVersion                bool

	db          *badger.DB
	chargePoint ocpp16.ChargePoint
	handler     *ChargePointHandler
	stopC       chan struct{}

	ll        = log.StandardLogger()
	appLogger = ll.WithContext(context.Background())

	chargePointId string
)

func init() {
	time.Local = time.UTC
}

func main() {
	// listen to quit signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)
	defer signal.Stop(signals)

	flag.StringVar(&chargePointId, "cp", "", "charge point id")
	flag.StringVar(&csUrl, "cs", "", "central system url")
	flag.StringVar(&controlPort, "control-port", "", "control server port (default: random)")
	flag.StringVar(&dbPath, "db", "db", "db path")
	flag.BoolVar(&showVersion, "version", false, "show version")

	flag.Parse()
	if showVersion {
		fmt.Println("Current App Version:", appVersion)
		os.Exit(0)
	}

	if chargePointId == "" {
		println("missing charge point id")
		flag.Usage()
		os.Exit(1)
	}
	if csUrl == "" {
		println("missing central system url")
		flag.Usage()
		os.Exit(1)
	}

	appLogger = appLogger.WithField("cp", chargePointId)

	dbPath := filepath.Join(dbPath, chargePointId)
	badgerDB, err := badger.Open(badger.DefaultOptions(dbPath))
	if err != nil {
		log.Fatal(err)
	}
	defer badgerDB.Close()
	db = badgerDB

	// store setup configuration
	if err := db.Update(func(txn *badger.Txn) error {
		// txn.Set([]byte("SecurityProfile"), []byte(fmt.Sprintf("%d", NoSecurityProfile)))
		// txn.Delete([]byte("AuthorizationKey"))
		// txn.Delete([]byte("root_certificate"))
		// txn.Delete([]byte("current_transaction_id"))

		txn.Set([]byte("started_at"), []byte(time.Now().Format(time.RFC3339)))
		txn.Set([]byte("charge_point_id"), []byte(chargePointId))
		txn.Set([]byte("cs_url"), []byte(csUrl))
		txn.Set([]byte("cp_version"), []byte(appVersion))
		txn.Set([]byte("db_path"), []byte(dbPath))
		SetIfNotExistsTX(txn, "SecurityProfile", fmt.Sprintf("%d", NoSecurityProfile))
		SetIfNotExistsTX(txn, "MeterValueSampleInterval", "300")
		SetIfNotExistsTX(txn, "MeterValuesSampledData", "Energy.Active.Import.Register")
		SetIfNotExistsTX(txn, "CertificateStoreMaxLength", "1")
		SetIfNotExistsTX(txn, "default_heartbeat_interval", "300")
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	httpPort := startHttpServer()
	appLogger = appLogger.WithField("control_port", httpPort)

	ws.SetLogger(ll)

	client, err := setUpSecurityOnWsClient()
	if err != nil {
		appLogger.WithError(err).Fatalln("setUpSecurityOnWsClient")
	}

	if err := startChargePoint(client); err != nil {
		appLogger.WithError(err).Fatalln("startChargePoint")
	}

	<-signals
	go func() {
		<-signals
		fmt.Println("Forcefully shutting down...")

		closeStopC()

		db.Update(func(txn *badger.Txn) error {
			return txn.Set([]byte("stopped_at"), []byte(time.Now().Format(time.RFC3339)))
		})
		os.Exit(2)
	}()

	fmt.Println("Gracefully shutting down...")

	db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("stopped_at"), []byte(time.Now().Format(time.RFC3339)))
	})

	closeStopC()

	if chargePoint.IsConnected() {
		chargePoint.Stop()
	}
}

func closeStopC() {
	defer func() {
		recover()
	}()
	close(stopC)

}

func bootCharger() error {
	if chargePoint.IsConnected() {
		return errors.New("charge point already connected")
	}
	client, err := setUpSecurityOnWsClient()
	if err != nil {
		return err
	}
	if err := startChargePoint(client); err != nil {
		return err
	}
	return nil
}

func stopCharger() error {
	if !chargePoint.IsConnected() {
		return errors.New("charge point not connected")
	}
	closeStopC()
	chargePoint.Stop()
	return nil
}

func rebootCharger() error {
	if chargePoint.IsConnected() {
		closeStopC()
		chargePoint.Stop()
	}
	appLogger.Infoln("Charge Point stopped")
	client, err := setUpSecurityOnWsClient()
	if err != nil {
		return err
	}
	if err := startChargePoint(client); err != nil {
		return err
	}
	return nil
}

func setUpSecurityOnWsClient() (*ws.Client, error) {
	client := ws.NewClient()

	err := db.View(func(txn *badger.Txn) error {
		profile := MustGetIntKeyTX(txn, "SecurityProfile")

		if profile == NoSecurityProfile {
			return nil
		} else if profile == BasicSecurityProfile {
			password, err := GetKeyValueTX(txn, "AuthorizationKey")
			if err != nil {
				return err
			}
			if password == "" {
				return errors.New("password is not set for this profile")
			}
			client.SetBasicAuth(chargePointId, password)

		} else if profile == BasicSecurityWithTLSProfile {
			if !strings.HasPrefix(csUrl, "wss://") {
				return errors.New("central system url must be wss:// for this profile")
			}

			password, err := GetKeyValueTX(txn, "AuthorizationKey")
			if err != nil {
				return err
			}
			if password == "" {
				return errors.New("password is not set for this profile")
			}
			rootCert, err := GetKeyValueTX(txn, "root_certificate")
			if err != nil {
				return err
			}
			if rootCert == "" {
				return errors.New("not all security profile keys are set")
			}
			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM([]byte(rootCert)) {
				return errors.New("failed to append root certificate")
			}

			// we need to create a new tls client
			client = ws.NewTLSClient(&tls.Config{
				RootCAs: certPool,
			})

			client.SetBasicAuth(chargePointId, password)

		} else {
			return fmt.Errorf("security profile: %d not supported", profile)
		}
		return nil
	})
	return client, err
}

func startChargePoint(wsClient *ws.Client) error {
	chargePoint = ocpp16.NewChargePoint(chargePointId, nil, wsClient)

	handler = &ChargePointHandler{}
	chargePoint.SetCoreHandler(handler)

	chargePoint.SetSecurityHandler(handler)
	chargePoint.SetLogHandler(handler)
	chargePoint.SetExtendedTriggerMessageHandler(handler)
	chargePoint.SetSecureFirmwareHandler(handler)
	chargePoint.SetCertificateHandler(handler)

	// Connects to central system
	if err := chargePoint.Start(csUrl); err != nil {
		return err
	}

	// Charger Operation
	if err := bootNotification(); err != nil {
		return err
	}

	stopC = make(chan struct{})

	go func() {
		for {
			interval := MustGetIntKey("default_heartbeat_interval")
			time.Sleep(time.Duration(interval) * time.Second)

			select {
			case <-stopC:
				appLogger.Debugln("stop signal received in heartbeat")
				return
			default:
			}

			_, err := chargePoint.Heartbeat()
			if err != nil {
				appLogger.WithError(err).Debugln("Heartbeat error")
				continue
			}
			appLogger.Println("Heartbeat sent to central system")
		}
	}()

	go func() {
		for range time.Tick(20 * time.Minute) {
			select {
			case <-stopC:
				appLogger.Debugln("stop signal received in heartbeat")
				return
			default:
			}

			_, err := chargePoint.DiagnosticsStatusNotification(firmware.DiagnosticsStatusIdle)

			if err != nil {
				appLogger.WithError(err).Debugln("DiagnosticsStatusNotification")
				continue
			}
			appLogger.Debugln("DiagnosticsStatusNotification", firmware.DiagnosticsStatusIdle)
		}
	}()

	return nil
}
