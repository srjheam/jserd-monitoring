package collector

import (
	"context"
	"errors"
	"fmt"
	"server/internal/audit"
	"server/internal/config"
	"server/internal/normalizer"
	"time"
)

const (
	CycleStatusOK        = "succeeded"
	CycleStatusFailed    = "failed"
	artifactKindArticles = "articles"
	artifactKindReviews  = "reviews"
	artifactKindViews    = "views"
)

var ErrAuthenticationFailed = errors.New("authentication failed")

type Collector struct {
	Config     config.Config
	Clock      func() time.Time
	LiveClient LiveClient
}

type LiveClient interface {
	Run(ctx context.Context, cfg config.Config) ([]audit.RawArtifact, error)
}

type FailingLiveClient struct {
	Err error
}

func (c FailingLiveClient) Run(context.Context, config.Config) ([]audit.RawArtifact, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	return nil, ErrAuthenticationFailed
}

type CycleResult struct {
	SourceMode   config.SourceMode
	StartedAt    time.Time
	FinishedAt   time.Time
	Status       string
	Manifest     audit.Manifest
	RawArtifacts []audit.RawArtifact
	Normalized   normalizer.NormalizedCycle
}

// addArtifact records one artifact, in the result and in the manifest alike.
func (r *CycleResult) addArtifact(artifact audit.RawArtifact) {
	r.RawArtifacts = append(r.RawArtifacts, artifact)
	r.Manifest.Artifacts = append(r.Manifest.Artifacts, artifact)
}

// addError records one failure in the manifest, redacted.
func (r *CycleResult) addError(err error) {
	r.Manifest.Errors = append(r.Manifest.Errors, audit.RedactError(err.Error()))
}

func (c Collector) Run(ctx context.Context, cfgOverride ...config.Config) (result CycleResult, err error) {
	cfg := c.Config
	if len(cfgOverride) > 0 {
		cfg = cfgOverride[0]
	}

	now := c.now()
	result = CycleResult{
		SourceMode: cfg.SourceMode,
		StartedAt:  now,
		Status:     CycleStatusFailed,
		Manifest: audit.Manifest{
			SourceMode:  string(cfg.SourceMode),
			CollectedAt: now,
		},
	}
	defer func() {
		result.FinishedAt = c.now()
	}()

	if err := config.Validate(cfg); err != nil {
		result.addError(err)
		return result, err
	}
	err = c.runSource(ctx, cfg, &result)
	return result, err
}

// runSource collects the cycle from the configured source.
func (c Collector) runSource(ctx context.Context, cfg config.Config, result *CycleResult) error {
	switch cfg.SourceMode {
	case config.SourceModeFixtures:
		return c.runFixtures(ctx, cfg, result)
	case config.SourceModeLive:
		return c.runLive(ctx, cfg, result)
	}
	err := fmt.Errorf("unsupported source mode %q", cfg.SourceMode)
	result.Manifest.Errors = append(result.Manifest.Errors, err.Error())
	return err
}

func (c Collector) now() time.Time {
	if c.Clock != nil {
		return c.Clock()
	}
	return time.Now().UTC()
}
