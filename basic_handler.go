package main

import (
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
)

type ChargePointHandler struct{}

func (handler *ChargePointHandler) OnChangeAvailability(request *core.ChangeAvailabilityRequest) (confirmation *core.ChangeAvailabilityConfirmation, err error) {
	appLogger.Println("OnChangeAvailability", request.ConnectorId, request.Type)
	return core.NewChangeAvailabilityConfirmation(core.AvailabilityStatusAccepted), nil
}

func (handler *ChargePointHandler) OnClearCache(request *core.ClearCacheRequest) (confirmation *core.ClearCacheConfirmation, err error) {
	appLogger.Println("OnClearCache", request.GetFeatureName())
	return core.NewClearCacheConfirmation(core.ClearCacheStatusAccepted), nil
}

func (handler *ChargePointHandler) OnDataTransfer(request *core.DataTransferRequest) (confirmation *core.DataTransferConfirmation, err error) {
	appLogger.Println("OnDataTransfer", request.VendorId, request.MessageId, request.Data)
	return core.NewDataTransferConfirmation("someData"), nil
}

func (handler *ChargePointHandler) OnReset(request *core.ResetRequest) (confirmation *core.ResetConfirmation, err error) {
	appLogger.Println("OnReset", request.Type)
	return core.NewResetConfirmation(core.ResetStatusAccepted), nil
}
