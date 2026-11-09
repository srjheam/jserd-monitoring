package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// errInvalidValue marks an environment value the config cannot use. The caller
// turns it into the message for that setting.
var errInvalidValue = errors.New("invalid value")

// parsedSetting is one environment value that is not plain text.
type parsedSetting struct {
	key      string
	fallback string
	message  string
	store    func(*Config, string) error
}

// parsedSettings are read in this order, so the first bad value decides the
// message Load reports.
var parsedSettings = []parsedSetting{
	{"EDITORIAL_RN11_FALLBACK_DAYS", "7", "EDITORIAL_RN11_FALLBACK_DAYS must be a positive integer", storeFallbackDays},
	{"ALLOW_TEST_DELIVERY", "true", "ALLOW_TEST_DELIVERY must be true or false", storeAllowTestDelivery},
	{"SMTP_PORT", "0", "SMTP_PORT must be an integer between 1 and 65535 for smtp mode", storeSMTPPort},
	{"SMTP_TLS", "false", "SMTP_TLS must be true or false", storeSMTPTLS},
	{"OJS_REQUEST_TIMEOUT_SECONDS", "30", "OJS_REQUEST_TIMEOUT_SECONDS must be a positive integer", storeRequestTimeout},
}

// addParsedValues reads every setting that is not plain text onto the config.
func addParsedValues(lookup GetenvFunc, cfg *Config) error {
	for _, setting := range parsedSettings {
		raw := strings.TrimSpace(valueOrDefault(lookup, setting.key, setting.fallback))
		if err := setting.store(cfg, raw); err != nil {
			return fmt.Errorf("%s", setting.message)
		}
	}
	return nil
}

// storeFallbackDays reads the RN11 fallback window, in days.
func storeFallbackDays(cfg *Config, raw string) error {
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 {
		return errInvalidValue
	}
	cfg.EditorialRN11FallbackDays = days
	return nil
}

// storeAllowTestDelivery reads whether this build accepts the intercepting
// transport.
func storeAllowTestDelivery(cfg *Config, raw string) error {
	allowed, err := strconv.ParseBool(raw)
	if err != nil {
		return errInvalidValue
	}
	cfg.AllowTestDelivery = allowed
	return nil
}

// storeSMTPPort reads the port of the SMTP server.
func storeSMTPPort(cfg *Config, raw string) error {
	port, err := strconv.Atoi(raw)
	if err != nil || port < 0 || port > 65535 {
		return errInvalidValue
	}
	cfg.SMTPPort = port
	return nil
}

// storeSMTPTLS reads whether the SMTP connection uses TLS.
func storeSMTPTLS(cfg *Config, raw string) error {
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return errInvalidValue
	}
	cfg.SMTPTLS = enabled
	return nil
}

// storeRequestTimeout reads how long one OJS request can take.
func storeRequestTimeout(cfg *Config, raw string) error {
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return errInvalidValue
	}
	cfg.RequestTimeout = time.Duration(seconds) * time.Second
	return nil
}
