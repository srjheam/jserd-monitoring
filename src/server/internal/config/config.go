package config

import (
	"os"
	"strings"
	"time"
)

type (
	SourceMode   string
	DeliveryMode string
)

const (
	SourceModeLive     SourceMode   = "live"
	SourceModeFixtures SourceMode   = "fixtures"
	DeliveryModeTest   DeliveryMode = "test"
	DeliveryModeSMTP   DeliveryMode = "smtp"
)

type Config struct {
	SourceMode                SourceMode
	BaseURL                   string
	JournalContext            string
	Username                  string
	Password                  string
	RequestTimeout            time.Duration
	FixturesDir               string
	ArtifactRoot              string
	AllowTestDelivery         bool
	DeliveryMode              DeliveryMode
	SMTPHost                  string
	SMTPPort                  int
	SMTPUsername              string
	SMTPPassword              string
	SMTPTLS                   bool
	SMTPFromAddress           string
	SMTPReplyTo               string
	OJSCollectionCron         string
	EmailWorkerCron           string
	SummaryCron               string
	EditorialTimezone         string
	EditorialRN11FallbackDays int
	OJSChiefEditorGroupID     string
	OJSChiefEditorGroupName   string
	AppURL                    string
}

type GetenvFunc func(string) (string, bool)

func Load(getenv ...GetenvFunc) (Config, error) {
	lookup := lookupOrEnv(getenv)
	cfg := Config{
		SourceMode:              SourceMode(valueOrDefault(lookup, "OJS_SOURCE_MODE", string(SourceModeFixtures))),
		BaseURL:                 strings.TrimSpace(valueOrDefault(lookup, "OJS_BASE_URL", "")),
		JournalContext:          strings.TrimSpace(valueOrDefault(lookup, "OJS_JOURNAL_CONTEXT", "jserd")),
		Username:                strings.TrimSpace(valueOrDefault(lookup, "OJS_USERNAME", "")),
		Password:                valueOrDefault(lookup, "OJS_PASSWORD", ""),
		FixturesDir:             strings.TrimSpace(valueOrDefault(lookup, "OJS_FIXTURES_DIR", "../../data/examples")),
		ArtifactRoot:            strings.TrimSpace(valueOrDefault(lookup, "OJS_ARTIFACT_ROOT", "../../.runtime/pb_data/collector_runs")),
		DeliveryMode:            DeliveryMode(strings.TrimSpace(valueOrDefault(lookup, "DELIVERY_MODE", string(DeliveryModeTest)))),
		SMTPHost:                strings.TrimSpace(valueOrDefault(lookup, "SMTP_HOST", "")),
		SMTPUsername:            strings.TrimSpace(valueOrDefault(lookup, "SMTP_USERNAME", "")),
		SMTPPassword:            valueOrDefault(lookup, "SMTP_PASSWORD", ""),
		SMTPFromAddress:         strings.TrimSpace(valueOrDefault(lookup, "SMTP_FROM_ADDRESS", "")),
		SMTPReplyTo:             strings.TrimSpace(valueOrDefault(lookup, "SMTP_REPLY_TO", "")),
		OJSCollectionCron:       strings.TrimSpace(valueOrDefault(lookup, "OJS_COLLECTION_CRON", "0 * * * *")),
		EmailWorkerCron:         strings.TrimSpace(valueOrDefault(lookup, "EMAIL_WORKER_CRON", "* * * * *")),
		SummaryCron:             strings.TrimSpace(valueOrDefault(lookup, "SUMMARY_CRON", "0 9 * * 1")),
		EditorialTimezone:       strings.TrimSpace(valueOrDefault(lookup, "EDITORIAL_TIMEZONE", "America/Sao_Paulo")),
		OJSChiefEditorGroupID:   strings.TrimSpace(valueOrDefault(lookup, "OJS_CHIEF_EDITOR_GROUP_ID", "")),
		OJSChiefEditorGroupName: strings.TrimSpace(valueOrDefault(lookup, "OJS_CHIEF_EDITOR_GROUP_NAME", "")),
		AppURL:                  strings.TrimSpace(valueOrDefault(lookup, "APP_URL", "")),
	}
	if err := addParsedValues(lookup, &cfg); err != nil {
		return Config{}, err
	}
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// lookupOrEnv is the environment reader to use: the one the caller passed, and
// the real environment otherwise.
func lookupOrEnv(getenv []GetenvFunc) GetenvFunc {
	if len(getenv) > 0 && getenv[0] != nil {
		return getenv[0]
	}
	return os.LookupEnv
}

func valueOrDefault(getenv GetenvFunc, key, fallback string) string {
	value, ok := getenv(key)
	if !ok {
		return fallback
	}
	return value
}
