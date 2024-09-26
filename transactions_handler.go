package main

import (
	"strconv"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/lorenzodonini/ocpp-go/ocpp"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
)

func (handler *ChargePointHandler) OnRemoteStartTransaction(request *core.RemoteStartTransactionRequest) (confirmation *core.RemoteStartTransactionConfirmation, err error) {
	connectorId := request.ConnectorId
	if connectorId == nil {
		return core.NewRemoteStartTransactionConfirmation(
			types.RemoteStartStopStatusRejected), err
	}

	if isTxRunning() {
		appLogger.
			WithField("idTag", request.IdTag).
			WithField("connectorId", *connectorId).
			Println("Transaction already running")

		return core.NewRemoteStartTransactionConfirmation(
			types.RemoteStartStopStatusRejected), err
	}

	appLogger.Infoln("Starting Transaction", request.IdTag, connectorId)

	startEnergyValue := MustGetIntKey(EnergyKey)
	req := core.NewStartTransactionRequest(*connectorId,
		request.IdTag,
		startEnergyValue,
		types.NewDateTime(time.Now()))

	err = chargePoint.SendRequestAsync(req, func(resp ocpp.Response, protoError error) {
		if conf, ok := resp.(*core.StartTransactionConfirmation); ok {
			tagInfo := conf.IdTagInfo

			switch tagInfo.Status {
			case types.AuthorizationStatusAccepted:
				setTxIdTag(request.IdTag)
				setTxId(conf.TransactionId, *connectorId)

				go handler.RunRemoteScenario()

				appLogger.Infoln("Transaction started", tagInfo.Status, conf.TransactionId)
				return
			default:
				appLogger.Println("Transaction won't start", tagInfo.Status)
			}
			return
		}

		appLogger.Println("StartTransactionConfirmation", resp, protoError)
	})

	return core.NewRemoteStartTransactionConfirmation(
		types.RemoteStartStopStatusAccepted), err
}

func (handler *ChargePointHandler) OnRemoteStopTransaction(request *core.RemoteStopTransactionRequest) (confirmation *core.RemoteStopTransactionConfirmation, err error) {
	appLogger.Infoln("OnRemoteStopTransaction", request.TransactionId)

	if !isTxRunning() {
		appLogger.Println("No transaction running")
		return core.NewRemoteStopTransactionConfirmation(types.RemoteStartStopStatusRejected), nil
	}

	txId := currentTxId()
	connectorId := currentTxConnectorId()

	req := core.NewStopTransactionRequest(connectorId,
		types.NewDateTime(time.Now()), request.TransactionId)

	req.Reason = core.ReasonEVDisconnected
	req.IdTag = currentTxIdTag()
	req.MeterStop = MustGetIntKey(EnergyKey)

	err = chargePoint.SendRequestAsync(req, func(resp ocpp.Response, protoError error) {
		if conf, ok := resp.(*core.StopTransactionConfirmation); ok {
			tagInfo := conf.IdTagInfo

			switch tagInfo.Status {
			case types.AuthorizationStatusAccepted:

				go func() {
					handler.StopRemoteScenario()
					resetCurrentTx()
				}()

				appLogger.Infoln("Transaction stopped", tagInfo.Status, request.TransactionId)
				return
			default:
				appLogger.Println("Transaction won't stop", txId, tagInfo.Status)
			}
			return

		}
		appLogger.Println("StopTransactionConfirmation", resp, protoError)
	})

	return core.NewRemoteStopTransactionConfirmation(types.RemoteStartStopStatusAccepted), err
}

func (handler *ChargePointHandler) OnUnlockConnector(request *core.UnlockConnectorRequest) (confirmation *core.UnlockConnectorConfirmation, err error) {
	connectorId := request.ConnectorId
	appLogger.Println("OnUnlockConnector", connectorId)

	go func() {
		setCurrentTxConnectorId(connectorId)
		statusNotification(core.ChargePointStatusPreparing, 0)
	}()

	go func() {
		time.Sleep(2 * time.Minute)
		if !isTxRunning() {
			setCurrentTxConnectorId(0)
			statusNotification(core.ChargePointStatusAvailable, 0)
			return
		}
	}()
	return core.NewUnlockConnectorConfirmation(core.UnlockStatusUnlocked), nil
}

func isTxRunning() bool {
	ext, _ := KeyExists("current_transaction_id")
	return ext
}

func currentTxConnectorId() int {
	id, _ := GetIntKey("current_transaction_connector_id")
	return id
}

func setCurrentTxConnectorId(id int) error {
	return db.Update(func(txn *badger.Txn) error {
		txn.Set([]byte("current_transaction_connector_id"), []byte(strconv.Itoa(id)))
		return nil
	})
}

func currentTxId() int {
	id, _ := GetIntKey("current_transaction_id")
	return id
}

func setTxId(id, connectorId int) error {
	return db.Update(func(txn *badger.Txn) error {
		txn.Set([]byte("current_transaction_id"), []byte(strconv.Itoa(id)))
		txn.Set([]byte("current_transaction_connector_id"), []byte(strconv.Itoa(connectorId)))
		return nil
	})
}

func resetCurrentTx() error {
	return db.Update(func(txn *badger.Txn) error {
		txn.Delete([]byte("current_transaction_id"))
		txn.Delete([]byte("current_transaction_connector_id"))
		txn.Delete([]byte("current_transaction_idTag"))
		for _, key := range flushableMeterValues {
			txn.Delete([]byte(key))
		}
		return nil
	})
}

func currentTxIdTag() string {
	tag, _ := GetKeyValue("current_transaction_idTag")
	return tag
}

func setTxIdTag(tag string) error {
	return db.Update(func(txn *badger.Txn) error {
		txn.Set([]byte("current_transaction_idTag"), []byte(tag))
		return nil
	})
}
