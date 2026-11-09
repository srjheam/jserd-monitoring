package config

import (
	"strings"
	"testing"
	"time"
)

const (
	testSMTPFromAddress  = "noreply@example.test"
	envRequestTimeout    = "OJS_REQUEST_TIMEOUT_SECONDS"
	envSourceMode        = "OJS_SOURCE_MODE"
	envSMTPFromAddress   = "SMTP_FROM_ADDRESS"
	envDeliveryMode      = "DELIVERY_MODE"
	envAllowTestDelivery = "ALLOW_TEST_DELIVERY"
)

func TestValidateFixturesModeRequiresFixturesDir(t *testing.T) {
	cfg := Config{
		SourceMode:        SourceModeFixtures,
		FixturesDir:       "",
		SMTPFromAddress:   testSMTPFromAddress,
		AllowTestDelivery: true,
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected fixtures mode without OJS_FIXTURES_DIR to fail")
	}

	msg := err.Error()
	if !strings.Contains(msg, "OJS_FIXTURES_DIR") {
		t.Fatalf("expected error to mention OJS_FIXTURES_DIR, got %q", msg)
	}
	if strings.Contains(msg, "secret") || strings.Contains(msg, "cookie") {
		t.Fatalf("validation error leaked secret-like value: %q", msg)
	}
}

func TestValidateLiveModeRequiresOJSConnectionFields(t *testing.T) {
	cfg := Config{
		SourceMode:        SourceModeLive,
		BaseURL:           "https://journals-sol.sbc.org.br/index.php",
		JournalContext:    "jserd",
		Username:          "editor@example.test",
		Password:          "top-secret",
		RequestTimeout:    30 * time.Second,
		SMTPFromAddress:   testSMTPFromAddress,
		AllowTestDelivery: true,
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected complete live config to pass, got %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{name: "base url", mutate: func(c *Config) { c.BaseURL = "" }, want: "OJS_BASE_URL"},
		{name: "journal context", mutate: func(c *Config) { c.JournalContext = "" }, want: "OJS_JOURNAL_CONTEXT"},
		{name: "username", mutate: func(c *Config) { c.Username = "" }, want: "OJS_USERNAME"},
		{name: "password", mutate: func(c *Config) { c.Password = "" }, want: "OJS_PASSWORD"},
		{name: "timeout", mutate: func(c *Config) { c.RequestTimeout = 0 }, want: envRequestTimeout},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assertMissingFieldRejected(t, cfg, testCase.mutate, testCase.want)
		})
	}
}

func assertMissingFieldRejected(t *testing.T, cfg Config, mutate func(*Config), want string) {
	t.Helper()
	broken := cfg
	mutate(&broken)

	err := Validate(broken)
	if err == nil {
		t.Fatalf("expected missing %s to fail", want)
	}

	msg := err.Error()
	if !strings.Contains(msg, want) {
		t.Fatalf("expected error to mention %s, got %q", want, msg)
	}
	if strings.Contains(msg, cfg.Password) {
		t.Fatalf("validation error leaked password: %q", msg)
	}
}

func TestLoadRejectsUnknownModeAndInvalidTimeout(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "unknown mode",
			env: map[string]string{
				envSourceMode:      "auto",
				envSMTPFromAddress: testSMTPFromAddress,
			},
			want: envSourceMode,
		},
		{
			name: "invalid timeout",
			env: map[string]string{
				envSourceMode:         "live",
				"OJS_BASE_URL":        "https://journals-sol.sbc.org.br/index.php",
				"OJS_JOURNAL_CONTEXT": "jserd",
				"OJS_USERNAME":        "editor@example.test",
				"OJS_PASSWORD":        "top-secret",
				envRequestTimeout:     "0",
				envSMTPFromAddress:    testSMTPFromAddress,
			},
			want: envRequestTimeout,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assertLoadRejectsEnv(t, testCase.env, testCase.want)
		})
	}
}

func assertLoadRejectsEnv(t *testing.T, env map[string]string, want string) {
	t.Helper()
	_, err := Load(func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	})
	if err == nil {
		t.Fatal("expected invalid config to fail")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error to mention %s, got %q", want, err.Error())
	}
	if strings.Contains(err.Error(), "top-secret") {
		t.Fatalf("config error leaked password: %q", err.Error())
	}
}

func TestTestDeliveryRequiresValidFromAddress(t *testing.T) {
	base := Config{SourceMode: SourceModeFixtures, FixturesDir: "fixtures", DeliveryMode: DeliveryModeTest, AllowTestDelivery: true}
	for _, value := range []string{"", "invalid", "ok@example.test\r\nBcc: attacker@example.test"} {
		cfg := base
		cfg.SMTPFromAddress = value
		err := Validate(cfg)
		leaked := value != "" && strings.Contains(err.Error(), value)
		if err == nil || !strings.Contains(err.Error(), envSMTPFromAddress) || leaked {
			t.Fatalf("from=%q err=%v", value, err)
		}
	}
	base.SMTPFromAddress = "JSERD <noreply@example.test>"
	if err := Validate(base); err != nil {
		t.Fatalf("valid test sender rejected: %v", err)
	}
}

func TestLoadNotificationConfig(t *testing.T) {
	env := map[string]string{
		envDeliveryMode: "smtp", "SMTP_HOST": "smtp.example.test", "SMTP_PORT": "587",
		"SMTP_USERNAME": "mailer", "SMTP_PASSWORD": "sentinel-password", "SMTP_TLS": "true",
		envSMTPFromAddress: "JSERD <noreply@example.test>", "SMTP_REPLY_TO": "editorial@example.test",
	}
	cfg, err := Load(func(key string) (string, bool) { value, ok := env[key]; return value, ok })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DeliveryMode != DeliveryModeSMTP || cfg.SMTPPort != 587 || !cfg.SMTPTLS || cfg.OJSCollectionCron != "0 * * * *" || cfg.EmailWorkerCron != "* * * * *" {
		t.Fatalf("notification config=%#v", cfg)
	}
	broken := mapsClone(env)
	broken["SMTP_TLS"] = "false"
	_, err = Load(func(key string) (string, bool) { value, ok := broken[key]; return value, ok })
	if err == nil || strings.Contains(err.Error(), env["SMTP_PASSWORD"]) {
		t.Fatalf("unsafe validation error: %v", err)
	}
}

func TestValidateCronSchedules(t *testing.T) {
	for _, testCase := range []struct{ key, collection, worker string }{
		{key: "OJS_COLLECTION_CRON", collection: "15m", worker: "* * * * *"},
		{key: "EMAIL_WORKER_CRON", collection: "0 * * * *", worker: "60s"},
	} {
		_, err := Load(func(key string) (string, bool) {
			values := map[string]string{"OJS_COLLECTION_CRON": testCase.collection, "EMAIL_WORKER_CRON": testCase.worker}
			value, ok := values[key]
			return value, ok
		})
		if err == nil || !strings.Contains(err.Error(), testCase.key) {
			t.Fatalf("expected %s error, got %v", testCase.key, err)
		}
	}
}

func TestConfigEditorialDefaultsAndValidation(t *testing.T) {
	cfg, err := Load(func(key string) (string, bool) {
		values := map[string]string{envSMTPFromAddress: testSMTPFromAddress}
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SummaryCron != "0 9 * * 1" || cfg.EditorialTimezone != "America/Sao_Paulo" || cfg.EditorialRN11FallbackDays != 7 {
		t.Fatalf("editorial defaults=%#v", cfg)
	}
	for _, testCase := range []struct{ key, value string }{
		{key: "SUMMARY_CRON", value: "weekly"},
		{key: "EDITORIAL_TIMEZONE", value: "Mars/Olympus"},
		{key: "EDITORIAL_RN11_FALLBACK_DAYS", value: "0"},
	} {
		_, err := Load(func(key string) (string, bool) {
			values := map[string]string{envSMTPFromAddress: testSMTPFromAddress, testCase.key: testCase.value}
			value, ok := values[key]
			return value, ok
		})
		if err == nil || !strings.Contains(err.Error(), testCase.key) {
			t.Fatalf("%s=%q err=%v", testCase.key, testCase.value, err)
		}
	}
}

func TestAllowTestDeliveryFalseRejectsTestTransport(t *testing.T) {
	_, err := Load(func(key string) (string, bool) {
		values := map[string]string{envAllowTestDelivery: "false", envDeliveryMode: string(DeliveryModeTest), envSMTPFromAddress: testSMTPFromAddress}
		value, ok := values[key]
		return value, ok
	})
	if err == nil || !strings.Contains(err.Error(), envAllowTestDelivery) || !strings.Contains(err.Error(), envDeliveryMode) {
		t.Fatalf("expected ALLOW_TEST_DELIVERY=false/test rejection, got %v", err)
	}
}

func TestAllowTestDeliveryTrueAcceptsTestTransport(t *testing.T) {
	cfg, err := Load(func(key string) (string, bool) {
		values := map[string]string{envAllowTestDelivery: "true", envDeliveryMode: string(DeliveryModeTest), envSMTPFromAddress: testSMTPFromAddress}
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("expected ALLOW_TEST_DELIVERY=true to accept the test transport, got %v", err)
	}
	if !cfg.AllowTestDelivery {
		t.Fatalf("expected AllowTestDelivery to be true, got %#v", cfg)
	}
}

func TestAllowTestDeliveryAbsentDefaultsToTrue(t *testing.T) {
	cfg, err := Load(func(key string) (string, bool) {
		values := map[string]string{envDeliveryMode: string(DeliveryModeTest), envSMTPFromAddress: testSMTPFromAddress}
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("expected absent ALLOW_TEST_DELIVERY to default to true, got %v", err)
	}
	if !cfg.AllowTestDelivery {
		t.Fatalf("expected AllowTestDelivery to default to true, got %#v", cfg)
	}
}

func TestAllowTestDeliveryRejectsNonBoolean(t *testing.T) {
	_, err := Load(func(key string) (string, bool) {
		values := map[string]string{envAllowTestDelivery: "maybe", envSMTPFromAddress: testSMTPFromAddress}
		value, ok := values[key]
		return value, ok
	})
	if err == nil || !strings.Contains(err.Error(), envAllowTestDelivery) {
		t.Fatalf("expected non-boolean ALLOW_TEST_DELIVERY to fail, got %v", err)
	}
}

func mapsClone(source map[string]string) map[string]string {
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
