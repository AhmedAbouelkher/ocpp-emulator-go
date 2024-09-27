package main

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
)

func startHttpServer() string {
	mux := http.NewServeMux()

	type endpoint struct {
		path    string
		handler http.HandlerFunc
	}
	endpoints := []endpoint{
		{
			path: "/list-db",
			handler: func(w http.ResponseWriter, r *http.Request) {
				t := table.NewWriter()
				t.SetOutputMirror(w)
				t.AppendHeader(table.Row{"Key", "Value", "LTT"})
				db.View(func(txn *badger.Txn) error {
					opts := badger.DefaultIteratorOptions
					opts.PrefetchSize = 10
					it := txn.NewIterator(opts)
					defer it.Close()
					for it.Rewind(); it.Valid(); it.Next() {
						item := it.Item()
						k := item.Key()
						v, _ := item.ValueCopy(nil)
						if len(v) > 150 {
							v = []byte(fmt.Sprintf("%s...", v[:150]))
						}
						t.AppendRows([]table.Row{
							{string(k), string(v), item.ExpiresAt()},
						})
					}
					return nil
				})
				t.Render()
			},
		},
		{
			path: "/preparing",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !chargePoint.IsConnected() {
					w.Write([]byte("Charge Point not connected"))
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				connectorId, _ := strconv.Atoi(r.URL.Query().Get("connectorId"))
				statusNotification(core.ChargePointStatusAvailable, connectorId)
				time.Sleep(1 * time.Second)
				statusNotification(core.ChargePointStatusPreparing, connectorId)
				if connectorId == 0 {
					connectorId = currentTxConnectorId()
				}
				appLogger.Infoln("Status changed to", core.ChargePointStatusPreparing, "for connector", connectorId)
				w.WriteHeader(http.StatusNoContent)
			},
		},
		{
			path: "/ev-stop",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !chargePoint.IsConnected() {
					w.Write([]byte("Charge Point not connected"))
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				if !isTxRunning() {
					w.Write([]byte("No transaction running"))
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				ts := types.Now()
				txId := currentTxId()

				energy := MustGetIntKey(EnergyKey)

				conf, err := chargePoint.StopTransaction(
					energy,
					ts,
					txId,
					func(request *core.StopTransactionRequest) {
						request.Reason = core.ReasonEVDisconnected
						request.IdTag = currentTxIdTag()
					},
				)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				status := conf.IdTagInfo.Status
				switch status {
				case types.AuthorizationStatusAccepted:
					appLogger.Infoln("Transaction stopped", txId, status)
					go func() {
						handler.StopRemoteScenario()
						resetCurrentTx()
					}()
					return
				default:
					appLogger.Println("Transaction will not stop", txId, status)
				}
				w.WriteHeader(http.StatusNoContent)
			},
		},
		{
			path: "/start",
			handler: func(w http.ResponseWriter, r *http.Request) {
				err := bootCharger()
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
				w.Write([]byte("Charge Point started"))
			},
		},
		{
			path: "/stop",
			handler: func(w http.ResponseWriter, r *http.Request) {
				err := stopCharger()
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
				w.Write([]byte("Charge Point stopped"))
			},
		},
		{
			path: "/reboot",
			handler: func(w http.ResponseWriter, r *http.Request) {
				err := rebootCharger()
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
				w.Write([]byte("Charge Point rebooted"))
			},
		},
	}
	endpoints = append(endpoints, endpoint{
		path: "/list",
		handler: func(w http.ResponseWriter, r *http.Request) {
			value := "Available endpoints:\n"
			for _, v := range endpoints {
				value += fmt.Sprintf("\t%s\n", v.path)
			}
			w.Write([]byte(value))
		},
	})

	for _, e := range endpoints {
		mux.HandleFunc(e.path, e.handler)
	}

	if controlPort == "" {
		controlPort = "0"
	}

	listener, err := net.Listen("tcp", ":"+controlPort)
	if err != nil {
		appLogger.Fatalln("Error starting control server", err)
	}
	go http.Serve(listener, mux)

	port := listener.Addr().String()
	appLogger.Infoln("Control Server started on port", port)
	return port
}
