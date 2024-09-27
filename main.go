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
	wsClient := ws.NewClient()

	if err := setUpSecurityOnWsClient(wsClient); err != nil {
		appLogger.WithError(err).Fatalln("setUpSecurityOnWsClient")
	}

	if err := startChargePoint(wsClient); err != nil {
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

func setUpSecurityOnWsClient(client *ws.Client) error {
	return db.View(func(txn *badger.Txn) error {
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
			// rootCert, err := GetKeyValueTX(txn, "root_certificate")
			// if err != nil {
			// 	return err
			// }
			// if rootCert == "" {
			// 	return errors.New("not all security profile keys are set")
			// }
			rootCert := `-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----`
			certPool, err := x509.SystemCertPool()
			if err != nil {
				return err
			}
			if !certPool.AppendCertsFromPEM([]byte(rootCert)) {
				return errors.New("failed to append root certificate")
			}
			// we need to create a new tls client
			client = ws.NewTLSClient(&tls.Config{
				RootCAs:            certPool,
				InsecureSkipVerify: true,
			})
			client.SetBasicAuth(chargePointId, password)

		} else {
			return fmt.Errorf("security profile: %d not supported", profile)
		}
		return nil
	})
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

func bootCharger() error {
	if chargePoint.IsConnected() {
		return errors.New("charge point already connected")
	}
	wsClient := ws.NewClient()
	ws.SetLogger(ll)
	if err := setUpSecurityOnWsClient(wsClient); err != nil {
		return err
	}
	if err := startChargePoint(wsClient); err != nil {
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
	wsClient := ws.NewClient()
	ws.SetLogger(ll)
	if err := setUpSecurityOnWsClient(wsClient); err != nil {
		return err
	}
	if err := startChargePoint(wsClient); err != nil {
		return err
	}
	return nil
}
