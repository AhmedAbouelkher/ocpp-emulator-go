package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-faker/faker/v4"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/sirupsen/logrus"
)

func (h *ChargePointHandler) RunRemoteScenario() error {
	appLogger.Info("Starting/Resuming remote charging scenario")
	statusNotification(core.ChargePointStatusCharging, 0)

	for {
		meterValueIntervalInSeconds := MustGetIntKey("MeterValueSampleInterval")
		if meterValueIntervalInSeconds == 0 {
			meterValueIntervalInSeconds = 60
		}
		time.Sleep(time.Duration(meterValueIntervalInSeconds) * time.Second)

		db.Update(func(txn *badger.Txn) error {
			IncrementKeyTX(txn, EnergyKey, fakeNumber(200, 1000))
			IncrementKeyTX(txn, InstantaneousTemperatureKey, fakeNumber(20, 50))
			IncrementKeyTX(txn, BatteryPercentageKey, fakeNumber(0, int(time.Now().Unix())%100))
			p, v, c := generateFakePAV()
			IncrementKeyTX(txn, InstantaneousPowerKey, p)
			IncrementKeyTX(txn, InstantaneousVoltageKey, v)
			IncrementKeyTX(txn, InstantaneousCurrentKey, c)
			// ic, _ := GetIntKeyTX(txn, InstantaneousCurrentOfferedKey)
			// IncrementKeyTX(txn, InstantaneousCurrentOfferedKey, int(int(math.Max(
			// 	float64(ic),
			// 	float64(c),
			// ))))
			return nil
		})

		if !isTxRunning() {
			break
		}
		logFields := genMeterValues()
		logFields["connector_id"] = currentTxConnectorId()
		logFields["transaction_id"] = currentTxId()
		logFields["interval"] = meterValueIntervalInSeconds

		if err := sendMeterValues(); err != nil {
			appLogger.WithError(err).
				WithFields(logFields).
				Error("Error sending Energy meter value")
		} else {
			appLogger.WithFields(logFields).Info("Energy meter value sent")
		}
	}
	return nil
}

func genMeterValues() logrus.Fields {
	fields := logrus.Fields{}
	db.View(func(txn *badger.Txn) error {
		fields["energy_meter_value"] = MustGetIntKeyTX(txn, EnergyKey)
		fields["instantaneous_power"] = MustGetIntKeyTX(txn, InstantaneousPowerKey)
		fields["instantaneous_voltage"] = MustGetIntKeyTX(txn, InstantaneousVoltageKey)
		fields["instantaneous_current"] = MustGetIntKeyTX(txn, InstantaneousCurrentKey)
		// fields["instantaneous_current_offered"] = MustGetIntKeyTX(txn, InstantaneousCurrentOfferedKey)
		fields["instantaneous_temperature"] = MustGetIntKeyTX(txn, InstantaneousTemperatureKey)
		fields["battery_percentage"] = MustGetIntKeyTX(txn, BatteryPercentageKey)
		return nil
	})
	return fields
}

func (h *ChargePointHandler) StopRemoteScenario() error {
	statusNotification(core.ChargePointStatusFinishing, 0)
	time.Sleep(1 * time.Second)
	statusNotification(core.ChargePointStatusAvailable, 0)
	return nil
}

func bootNotification() error {
	result, err := chargePoint.BootNotification(
		faker.LastName(), faker.FirstName(),
		func(request *core.BootNotificationRequest) {
			request.ChargePointSerialNumber = faker.CCNumber()
			request.MeterSerialNumber = faker.CCNumber()
			request.MeterType = faker.CCNumber()
			request.Iccid = faker.CCNumber()
			request.FirmwareVersion = "v1.0.0"
		})
	if err != nil {
		return err
	}
	if result.Status != core.RegistrationStatusAccepted {
		appLogger.Println("BootNotification rejected", result.Status)
	}
	return db.Update(func(txn *badger.Txn) error {
		i := fmt.Sprintf("%d", result.Interval)
		return txn.Set([]byte("default_heartbeat_interval"), []byte(i))
	})
}

func statusNotification(s core.ChargePointStatus, connectorId int) error {
	if connectorId == 0 {
		connectorId = currentTxConnectorId()
	}
	_, err := chargePoint.StatusNotification(
		connectorId, core.NoError, s,
		func(request *core.StatusNotificationRequest) {
			request.Info = faker.MonthName()
			request.Info = faker.MonthName()
			request.VendorId = "vendor_" + faker.CCNumber()
			request.Timestamp = types.NewDateTime(time.Now())
		},
	)
	return err
}

func sendMeterValues() error {
	sampledValues := []types.SampledValue{}

	var rawData string

	if err := db.View(func(txn *badger.Txn) error {
		data, err := GetKeyValueTX(txn, "MeterValuesSampledData")
		if err != nil {
			return err
		}
		rawData = data
		return nil
	}); err != nil {
		return err
	}

	meterValuesSampledData := strings.Split(rawData, ",")

	err := db.View(func(txn *badger.Txn) error {

		for _, k := range meterValuesSampledData {
			value := types.SampledValue{
				Format:   types.ValueFormatRaw,
				Context:  types.ReadingContextSamplePeriodic,
				Location: types.LocationOutlet,
				Phase:    types.PhaseL1,
			}

			switch types.Measurand(k) {
			case types.MeasurandEnergyActiveImportRegister:
				randomTrigger(func() {
					value.Value = fmt.Sprintf("%d", MustGetIntKeyTX(txn, EnergyKey))
					value.Unit = types.UnitOfMeasureWh
					value.Measurand = types.MeasurandEnergyActiveImportRegister
				})

			case types.MeasurandPowerActiveImport:
				randomTrigger(func() {
					value.Value = fmt.Sprintf("%d", MustGetIntKeyTX(txn, InstantaneousPowerKey))
					value.Unit = types.UnitOfMeasureW
					value.Measurand = types.MeasurandPowerActiveImport
				})

			case types.MeasurandCurrentImport:
				randomTrigger(func() {
					value.Value = fmt.Sprintf("%d", MustGetIntKeyTX(txn, InstantaneousCurrentKey))
					value.Unit = types.UnitOfMeasureA
					value.Measurand = types.MeasurandCurrentImport
				})

			// case types.MeasurandCurrentOffered:
			// 	randomTrigger(func() {
			// 		value.Value = fmt.Sprintf("%d", MustGetIntKeyTX(txn, InstantaneousCurrentOfferedKey))
			// 		value.Unit = types.UnitOfMeasureA
			// 		value.Measurand = types.MeasurandCurrentOffered
			// 	})

			case types.MeasurandVoltage:
				randomTrigger(func() {
					value.Value = fmt.Sprintf("%d", MustGetIntKeyTX(txn, InstantaneousVoltageKey))
					value.Unit = types.UnitOfMeasureV
					value.Measurand = types.MeasurandVoltage
				})

			case types.MeasurandTemperature:
				randomTrigger(func() {
					value.Value = fmt.Sprintf("%d", MustGetIntKeyTX(txn, InstantaneousTemperatureKey))
					value.Unit = types.UnitOfMeasureCelsius
					value.Measurand = types.MeasurandTemperature
				})

			case types.MeasurandSoC:
				randomTrigger(func() {
					value.Value = fmt.Sprintf("%d", MustGetIntKeyTX(txn, BatteryPercentageKey))
					value.Unit = types.UnitOfMeasurePercent
					value.Measurand = types.MeasurandSoC
				})
			}

			if value.Value == "" {
				continue
			}

			sampledValues = append(sampledValues, value)
		}

		return nil
	})
	if err != nil {
		return err
	}

	if len(sampledValues) == 0 {
		return nil
	}

	cid := currentTxConnectorId()

	_, err = chargePoint.MeterValues(
		cid,
		[]types.MeterValue{
			{
				Timestamp:    types.NewDateTime(time.Now()),
				SampledValue: sampledValues,
			},
		},
		func(request *core.MeterValuesRequest) {
			currentTxId := currentTxId()
			request.TransactionId = &currentTxId
		},
	)
	return err
}

func generateFakePAV() (int, int, int) {
	var voltage int
	var current int
	power := fakeNumber(1_000, 360_000) // W
	if power < 1_000 && power > 3_300 {
		voltage = 120
		current = fakeNumber(1, 12)
	} else if power >= 3_300 && power < 19_200 {
		voltage = fakeNumber(208, 240)
		current = fakeNumber(16, 80)
	} else {
		voltage = fakeNumber(380, 800)
		current = fakeNumber(80, 500)
	}
	return power, voltage, current
}

func fakeNumber(min, max int) int {
	v, _ := faker.RandomInt(min, max, 1)
	l := len(v)
	if l == 0 && l > 1 {
		return 0
	}
	return v[0]
}

func randomTrigger(fn func()) {
	if randomBoolean() {
		fn()
	}
}

func randomBoolean() bool {
	return rand.Intn(2)%2 == 0
}
