package main

const (
	EnergyKey = "meter_value__energy"

	InstantaneousPowerKey          = "meter_value__instantaneous_power"
	InstantaneousCurrentKey        = "meter_value__instantaneous_current"
	InstantaneousCurrentOfferedKey = "meter_value__instantaneous_current_offered"
	InstantaneousVoltageKey        = "meter_value__instantaneous_voltage"
	InstantaneousTemperatureKey    = "meter_value__instantaneous_temperature"
	BatteryPercentageKey           = "meter_value__battery_percentage"
)

const (
	NoSecurityProfile = iota
	BasicSecurityProfile
	BasicSecurityWithTLSProfile
)

var (
	flushableMeterValues = []string{
		InstantaneousPowerKey,
		InstantaneousCurrentKey,
		InstantaneousCurrentOfferedKey,
		InstantaneousVoltageKey,
		InstantaneousTemperatureKey,
		BatteryPercentageKey,
	}

	supportedConfigurationKeys = map[string]struct{}{
		"AuthorizeRemoteTxRequests":               {},
		"AuthorizationCacheEnabled":               {},
		"ClockAlignedDataInterval":                {},
		"ConnectionTimeOut":                       {},
		"ConnectorPhaseRotation":                  {},
		"GetConfigurationMaxKeys":                 {},
		"HeartbeatInterval":                       {},
		"LocalAuthorizeOffline":                   {},
		"LocalPreAuthorize":                       {},
		"MeterValuesAlignedData":                  {},
		"MeterValuesSampledData":                  {},
		"MeterValueSampleInterval":                {},
		"NumberOfConnectors":                      {},
		"ResetRetries":                            {},
		"StopTransactionOnEVSideDisconnect":       {},
		"StopTransactionOnInvalidId":              {},
		"StopTxnAlignedData":                      {},
		"StopTxnSampledData":                      {},
		"SupportedFeatureProfiles":                {},
		"TransactionMessageAttempts":              {},
		"TransactionMessageRetryInterval":         {},
		"UnlockConnectorOnEVSideDisconnect":       {},
		"WebSocketPingInterval":                   {},
		"LocalAuthListEnabled":                    {},
		"LocalAuthListMaxLength":                  {},
		"SendLocalListMaxLength":                  {},
		"ChargeProfileMaxStackLevel":              {},
		"ChargingScheduleAllowedChargingRateUnit": {},
		"ChargingScheduleMaxPeriods":              {},
		"MaxChargingProfilesInstalled":            {},
		"SupportedFileTransferProtocols":          {},
		"SecurityProfile":                         {},
		"CpoName":                                 {},
		"AdditionalRootCertificateCheck":          {},
		"CertificateStoreMaxLength":               {},
		"AuthorizationKey":                        {},
	}
)
