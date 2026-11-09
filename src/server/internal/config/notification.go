package config

// notificationChecks run in this order, so the first failure decides the
// message validateNotification reports.
var notificationChecks = []func(Config) error{validateSchedules, validateDeliveryMode, validateFromAddress, validateSMTP}

func validateNotification(cfg Config) error {
	for _, check := range notificationChecks {
		if err := check(cfg); err != nil {
			return err
		}
	}
	return nil
}
