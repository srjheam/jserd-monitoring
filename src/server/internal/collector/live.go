package collector

import (
	"context"
	"server/internal/audit"
	"server/internal/config"
	"server/internal/ojs"
)

const reportExtCSV = "csv"

// liveReport is one report export a live cycle downloads.
type liveReport struct {
	kind       string
	reportKind ojs.ReportKind
	ext        string
}

var liveReports = []liveReport{
	{kind: artifactKindArticles, reportKind: ojs.ReportKindArticles, ext: reportExtCSV},
	{kind: artifactKindReviews, reportKind: ojs.ReportKindReviews, ext: reportExtCSV},
	{kind: artifactKindViews, reportKind: ojs.ReportKindViews, ext: reportExtCSV},
}

// liveRun is one live collection cycle in progress: the signed-in client, where
// artifacts are stored, the result they fill, and the bodies the normalizer
// reads.
type liveRun struct {
	collector Collector
	cfg       config.Config
	client    *ojs.Client
	store     audit.ArtifactStore
	runID     string
	result    *CycleResult
	bodies    map[string][]byte
	refs      map[string]string
}

func (c Collector) runLive(ctx context.Context, cfg config.Config, result *CycleResult) error {
	if c.LiveClient != nil {
		return c.runLiveClient(ctx, cfg, result)
	}
	ojsClient, err := c.signInToOJS(ctx, cfg)
	if err != nil {
		result.addError(err)
		return err
	}
	run := &liveRun{
		collector: c,
		cfg:       cfg,
		client:    ojsClient,
		store:     audit.ArtifactStore{Root: cfg.ArtifactRoot},
		runID:     result.StartedAt.UTC().Format("20060102T150405Z"),
		result:    result,
		bodies:    map[string][]byte{},
		refs:      map[string]string{},
	}
	return run.collect(ctx)
}

// runLiveClient collects through the injected live client, which the tests
// supply in place of a real OJS server.
func (c Collector) runLiveClient(ctx context.Context, cfg config.Config, result *CycleResult) error {
	artifacts, err := c.LiveClient.Run(ctx, cfg)
	result.RawArtifacts = append(result.RawArtifacts, artifacts...)
	result.Manifest.Artifacts = append(result.Manifest.Artifacts, artifacts...)
	if err != nil {
		result.Status = CycleStatusFailed
		result.addError(err)
		return err
	}
	result.Status = CycleStatusOK
	return nil
}

// signInToOJS builds the OJS client and signs it in.
func (c Collector) signInToOJS(ctx context.Context, cfg config.Config) (*ojs.Client, error) {
	ojsClient, err := ojs.NewClient(cfg.BaseURL, cfg.JournalContext, cfg.RequestTimeout)
	if err != nil {
		return nil, err
	}
	if err := ojsClient.Login(ctx, cfg.Username, cfg.Password); err != nil {
		return nil, err
	}
	return ojsClient, nil
}

// collect downloads the reports and the submission list, then normalizes what
// came back.
func (r *liveRun) collect(ctx context.Context) error {
	if err := r.downloadReports(ctx); err != nil {
		return err
	}
	pages, err := r.downloadSubmissionPages(ctx)
	if err != nil {
		return err
	}
	return r.normalize(pages)
}

// downloadReports downloads every report export and stores it as an artifact.
func (r *liveRun) downloadReports(ctx context.Context) error {
	for _, report := range liveReports {
		if err := r.downloadReport(ctx, report); err != nil {
			return err
		}
	}
	return nil
}

// downloadReport downloads one report export. A failed download still produces
// a failed artifact, so the audit trail names it.
func (r *liveRun) downloadReport(ctx context.Context, report liveReport) error {
	body, sourceURL, err := r.client.DownloadReport(ctx, report.reportKind)
	artifact, storeErr := ojs.StoreDownloadedArtifact(r.store, r.runID, report.kind, report.ext, sourceURL, body, r.collector.now())
	if err != nil {
		artifact = audit.NewArtifact(report.kind, string(config.SourceModeLive), sourceURL, body, r.collector.now(), audit.ArtifactStatusFailed, err)
	} else if storeErr != nil {
		err = storeErr
	}
	r.result.addArtifact(artifact)
	if err != nil {
		r.result.addError(err)
		return err
	}
	r.bodies[report.kind] = body
	r.refs[report.kind] = artifact.PathOrURL
	return nil
}

// normalize turns the downloaded reports into the normalized cycle, and merges
// the REST metadata onto it.
func (r *liveRun) normalize(pages []ojs.SubmissionPage) error {
	normalized, err := normalizeFixtureArtifacts(r.bodies, r.refs, r.result.RawArtifacts, r.cfg, r.result.StartedAt)
	if err != nil {
		r.result.addError(err)
		return err
	}
	if err := applySubmissionMetadata(normalized.Submissions, pages); err != nil {
		r.result.addError(err)
		return err
	}
	r.result.Normalized = normalized
	r.result.Status = CycleStatusOK
	return nil
}
