package config

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase/tools/cron"
)

// validateSchedules checks every cron expression and the editorial timezone.
func validateSchedules(cfg Config) error {
	if err := validateCron("SUMMARY_CRON", cfg.SummaryCron, ""); err != nil {
		return err
	}
	if err := validateTimezone(cfg.EditorialTimezone); err != nil {
		return err
	}
	if cfg.EditorialRN11FallbackDays < 0 {
		return fmt.Errorf("EDITORIAL_RN11_FALLBACK_DAYS must be a positive integer")
	}
	return validateCollectionSchedules(cfg)
}

// validateCron checks one cron expression. An empty value falls back to the
// given default. An empty default means that the schedule is optional.
func validateCron(name, value, fallback string) error {
	if value == "" {
		value = fallback
	}
	if value == "" {
		return nil
	}
	if _, err := cron.NewSchedule(value); err != nil {
		return fmt.Errorf("%s must be a valid cron expression", name)
	}
	return nil
}

// validateTimezone checks the editorial timezone, which is optional.
func validateTimezone(name string) error {
	if name == "" {
		return nil
	}
	if _, err := time.LoadLocation(name); err != nil {
		return fmt.Errorf("EDITORIAL_TIMEZONE must be a valid IANA timezone")
	}
	return nil
}

// validateCollectionSchedules checks the two schedules that always run.
func validateCollectionSchedules(cfg Config) error {
	if err := validateCron("OJS_COLLECTION_CRON", cfg.OJSCollectionCron, "0 * * * *"); err != nil {
		return err
	}
	return validateCron("EMAIL_WORKER_CRON", cfg.EmailWorkerCron, "* * * * *")
}
