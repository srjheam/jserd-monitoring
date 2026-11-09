package config

import (
	"fmt"
	"strings"
)

func validateLive(cfg Config) error {
	if err := validateLiveOJSURL(cfg); err != nil {
		return err
	}
	return validateLiveCredentials(cfg)
}

// validateLiveOJSURL checks the address of the OJS server and the journal it
// serves.
func validateLiveOJSURL(cfg Config) error {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return fmt.Errorf("OJS_BASE_URL is required for live mode")
	}
	if !isWebURL(cfg.BaseURL) {
		return fmt.Errorf("OJS_BASE_URL must be an http or https URL")
	}
	if strings.TrimSpace(cfg.JournalContext) == "" {
		return fmt.Errorf("OJS_JOURNAL_CONTEXT is required for live mode")
	}
	return nil
}

// validateLiveCredentials checks the sign-in settings for the OJS server.
func validateLiveCredentials(cfg Config) error {
	if strings.TrimSpace(cfg.Username) == "" {
		return fmt.Errorf("OJS_USERNAME is required for live mode")
	}
	if cfg.Password == "" {
		return fmt.Errorf("OJS_PASSWORD is required for live mode")
	}
	if cfg.RequestTimeout <= 0 {
		return fmt.Errorf("OJS_REQUEST_TIMEOUT_SECONDS must be positive for live mode")
	}
	return nil
}
