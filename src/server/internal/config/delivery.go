package config

import (
	"fmt"
	"net/mail"
	"strings"
)

// deliveryMode is the configured delivery mode. An empty setting means test.
func deliveryMode(cfg Config) DeliveryMode {
	if cfg.DeliveryMode == "" {
		return DeliveryModeTest
	}
	return cfg.DeliveryMode
}

// validateDeliveryMode checks that the delivery mode is one this build allows.
func validateDeliveryMode(cfg Config) error {
	if err := validateTestDeliveryAllowed(cfg); err != nil {
		return err
	}
	mode := deliveryMode(cfg)
	if mode != DeliveryModeTest && mode != DeliveryModeSMTP {
		return fmt.Errorf("DELIVERY_MODE must be test or smtp")
	}
	return nil
}

// validateTestDeliveryAllowed rejects the intercepting transport in a build
// that must send real e-mail.
func validateTestDeliveryAllowed(cfg Config) error {
	if !cfg.AllowTestDelivery && deliveryMode(cfg) == DeliveryModeTest {
		return fmt.Errorf("DELIVERY_MODE=test is not allowed when ALLOW_TEST_DELIVERY=false")
	}
	return nil
}

// validateFromAddress checks the sender address of every e-mail.
func validateFromAddress(cfg Config) error {
	if !isSendableAddress(cfg.SMTPFromAddress) {
		return fmt.Errorf("SMTP_FROM_ADDRESS must be a valid email address")
	}
	return nil
}

// isSendableAddress reports whether an address can go in a From header: it is
// present, it carries no line break, and it parses as an address. A line break
// in an address injects a header.
func isSendableAddress(raw string) bool {
	if raw == "" || strings.ContainsAny(raw, "\r\n") {
		return false
	}
	parsed, err := mail.ParseAddress(raw)
	return err == nil && parsed.Address != ""
}
