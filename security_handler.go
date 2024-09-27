package main

import (
	"github.com/dgraph-io/badger/v4"
	"github.com/lorenzodonini/ocpp-go/ocpp"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/certificates"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/extendedtriggermessage"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/logging"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/securefirmware"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/security"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/lorenzodonini/ocpp-go/ocppj"
)

func (handler *ChargePointHandler) OnInstallCertificate(request *certificates.InstallCertificateRequest) (response *certificates.InstallCertificateResponse, err error) {
	appLogger.Println("InstallCertificate")

	if request.CertificateType == types.ManufacturerRootCertificate {
		appLogger.Println("Charge point does not support ManufacturerRootCertificate installation")
		return certificates.NewInstallCertificateResponse(certificates.CertificateStatusRejected), nil
	}

	if err := db.Update(func(txn *badger.Txn) error {
		certMaxStoreL, err := GetIntKeyTX(txn, "CertificateStoreMaxLength")
		if err != nil {
			return err
		}
		rootCertificate, err := GetKeyValueTX(txn, "root_certificate")
		if err != nil {
			return err
		}
		hasRootCertificate := rootCertificate != ""
		if hasRootCertificate && certMaxStoreL == 1 {
			appLogger.Println("no more space to install more certificates")
			return ocpp.NewError(ocppj.SecurityError, "no more space to install more certificates", "")
		}
		return txn.Set([]byte("root_certificate"), []byte(request.Certificate))
	}); err != nil {
		appLogger.WithError(err).Errorf("failed to install certificate")
		return certificates.NewInstallCertificateResponse(certificates.CertificateStatusRejected), err
	}

	return certificates.NewInstallCertificateResponse(certificates.CertificateStatusAccepted), nil
}

func (handler *ChargePointHandler) OnGetInstalledCertificateIds(request *certificates.GetInstalledCertificateIdsRequest) (response *certificates.GetInstalledCertificateIdsResponse, err error) {
	appLogger.Println("GetInstalledCertificateIds")
	return certificates.NewGetInstalledCertificateIdsResponse(certificates.GetInstalledCertificateStatusAccepted), nil
}

func (handler *ChargePointHandler) OnDeleteCertificate(request *certificates.DeleteCertificateRequest) (response *certificates.DeleteCertificateResponse, err error) {
	appLogger.Println("GetBaseReport")
	return certificates.NewDeleteCertificateResponse(certificates.DeleteCertificateStatusAccepted), nil
}

func (handler *ChargePointHandler) OnGetLog(request *logging.GetLogRequest) (response *logging.GetLogResponse, err error) {
	appLogger.Println("GetLog")
	return logging.NewGetLogResponse(logging.LogStatusAccepted), nil
}

func (handler *ChargePointHandler) OnSignedUpdateFirmware(request *securefirmware.SignedUpdateFirmwareRequest) (response *securefirmware.SignedUpdateFirmwareResponse, err error) {
	appLogger.Println("SignedUpdateFirmware")
	return securefirmware.NewSignedUpdateFirmwareResponse(securefirmware.UpdateFirmwareStatusAccepted), nil
}

func (handler *ChargePointHandler) OnExtendedTriggerMessage(request *extendedtriggermessage.ExtendedTriggerMessageRequest) (response *extendedtriggermessage.ExtendedTriggerMessageResponse, err error) {
	appLogger.Println("ExtendedTriggerMessage")
	return extendedtriggermessage.NewExtendedTriggerMessageResponse(extendedtriggermessage.ExtendedTriggerMessageStatusAccepted), nil
}

func (handler *ChargePointHandler) OnCertificateSigned(request *security.CertificateSignedRequest) (response *security.CertificateSignedResponse, err error) {
	appLogger.Println("CertificateSigned")
	return security.NewCertificateSignedResponse(security.CertificateSignedStatusAccepted), nil
}
