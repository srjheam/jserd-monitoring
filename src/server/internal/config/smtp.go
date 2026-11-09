package config

import "fmt"

// validateSMTP checks the SMTP settings. Test mode needs none of them.
func validateSMTP(cfg Config) error {
	if deliveryMode(cfg) == DeliveryModeTest {
		return nil
	}
	if err := validateSMTPServer(cfg); err != nil {
		return err
	}
	if (cfg.SMTPUsername == "") != (cfg.SMTPPassword == "") {
		return fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be configured together")
	}
	return validateSMTPTLS(cfg)
}

// validateSMTPServer checks the host and the port of the SMTP server.
func validateSMTPServer(cfg Config) error {
	if cfg.SMTPHost == "" {
		return fmt.Errorf("SMTP_HOST is required for smtp mode")
	}
	if cfg.SMTPPort <= 0 || cfg.SMTPPort > 65535 {
		return fmt.Errorf("SMTP_PORT is required for smtp mode")
	}
	return nil
}

// localHosts are the host names that name this machine.
var localHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}

// validateSMTPTLS requires TLS for every SMTP host that is not this machine.
func validateSMTPTLS(cfg Config) error {
	if localHosts[cfg.SMTPHost] || cfg.SMTPTLS {
		return nil
	}
	return fmt.Errorf("SMTP_TLS must be true for remote SMTP_HOST")
}
