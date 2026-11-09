package config

import (
	"fmt"
	"net/url"
	"strings"
)

func Validate(cfg Config) error {
	if err := validateNotification(cfg); err != nil {
		return err
	}
	if err := validateAppURL(cfg); err != nil {
		return err
	}
	return validateSource(cfg)
}

// validateSource checks the settings the chosen source mode needs.
func validateSource(cfg Config) error {
	switch cfg.SourceMode {
	case SourceModeFixtures:
		if strings.TrimSpace(cfg.FixturesDir) == "" {
			return fmt.Errorf("OJS_FIXTURES_DIR is required for fixtures mode")
		}
		return nil
	case SourceModeLive:
		return validateLive(cfg)
	}
	return fmt.Errorf("OJS_SOURCE_MODE must be live or fixtures")
}

// isWebURL reports whether a value is an http or https URL that names a host.
func isWebURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// validateAppURL checks APP_URL only when set: it points PocketBase's own
// mailer (chief_editors password reset/verification) at this app's public
// URL, but stays optional since it has no effect until SMTP_HOST is set too.
func validateAppURL(cfg Config) error {
	if cfg.AppURL == "" {
		return nil
	}
	if !isWebURL(cfg.AppURL) {
		return fmt.Errorf("APP_URL must be an http or https URL")
	}
	return nil
}
