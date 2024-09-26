package main

import (
	"errors"
	"strconv"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
)

func (handler *ChargePointHandler) OnChangeConfiguration(request *core.ChangeConfigurationRequest) (confirmation *core.ChangeConfigurationConfirmation, err error) {
	key := request.Key
	value := request.Value
	appLogger.Println("OnChangeConfiguration", key)
	if _, ok := supportedConfigurationKeys[key]; !ok {
		return core.NewChangeConfigurationConfirmation(core.ConfigurationStatusNotSupported), nil
	}

	requiresReboot := false

	switch key {
	case "SecurityProfile":
		v, _ := strconv.Atoi(value)
		if err := db.View(func(txn *badger.Txn) error {
			val := MustGetIntKeyTX(txn, "SecurityProfile")
			if v < val {
				return errors.New("cannot set a lower security profile")
			}
			if v == BasicSecurityProfile {
				password, err := GetKeyValueTX(txn, "AuthorizationKey")
				if err != nil {
					return err
				}
				if password == "" {
					return errors.New("not all security profile keys are set")
				}

				requiresReboot = true
			}
			return nil
		}); err != nil {
			appLogger.WithError(err).
				WithField("key", key).
				WithField("value", value).
				Error("Error updating configuration")
			return core.NewChangeConfigurationConfirmation(core.ConfigurationStatusRejected), err
		}
	}

	if err := db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(value))
	}); err != nil {
		appLogger.WithError(err).
			WithField("key", key).
			WithField("value", value).
			Error("Error updating configuration")
		return core.NewChangeConfigurationConfirmation(core.ConfigurationStatusRejected), err
	}

	if requiresReboot {
		appLogger.Info("Security profile change requires reboot")

		go func() {
			time.Sleep(1500 * time.Millisecond)

			err := rebootCharger()
			if err != nil {
				appLogger.WithError(err).Error("Error rebooting charger")
			}
		}()

	}

	return core.NewChangeConfigurationConfirmation(core.ConfigurationStatusAccepted), nil
}

func (handler *ChargePointHandler) OnGetConfiguration(request *core.GetConfigurationRequest) (confirmation *core.GetConfigurationConfirmation, err error) {
	keys := request.Key
	appLogger.Println("OnGetConfiguration", keys)
	unknownKeys := make([]string, 0)
	for _, key := range keys {
		if _, ok := supportedConfigurationKeys[key]; !ok {
			unknownKeys = append(unknownKeys, key)
		}
	}
	cKeys := []core.ConfigurationKey{}
	if err := db.View(func(txn *badger.Txn) error {
		for _, key := range keys {
			if _, ok := supportedConfigurationKeys[key]; !ok {
				continue
			}
			val, err := txn.Get([]byte(key))
			if err != nil {
				unknownKeys = append(unknownKeys, key)
				continue
			}
			v, _ := val.ValueCopy(nil)
			if len(v) == 0 {
				unknownKeys = append(unknownKeys, key)
				continue
			}
			value := string(v)
			cKeys = append(cKeys, core.ConfigurationKey{
				Key:   key,
				Value: &value,
			})
		}
		return nil
	}); err != nil {
		appLogger.WithError(err).Error("Error getting configuration")

		return nil, err
	}
	return &core.GetConfigurationConfirmation{UnknownKey: unknownKeys, ConfigurationKey: cKeys}, nil
}
