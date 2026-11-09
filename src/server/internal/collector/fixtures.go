package collector

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"server/internal/audit"
	"server/internal/config"
	"server/internal/normalizer"
	"time"
)

// fixtureFile is one report export the fixtures directory holds.
type fixtureFile struct {
	kind string
	file string
}

var fixtureFiles = []fixtureFile{
	{kind: artifactKindArticles, file: "articles-JSERD-20260512.csv"},
	{kind: artifactKindReviews, file: "reviews-20260512.csv"},
	{kind: artifactKindViews, file: "views-20260512.csv"},
}

// fixtureRun is one fixtures collection cycle in progress: where the fixture
// files live, the result they fill, and the bodies the normalizer reads.
type fixtureRun struct {
	collector   Collector
	cfg         config.Config
	fixturesDir string
	result      *CycleResult
	bodies      map[string][]byte
	refs        map[string]string
}

func (c Collector) runFixtures(ctx context.Context, cfg config.Config, result *CycleResult) error {
	run := &fixtureRun{
		collector:   c,
		cfg:         cfg,
		fixturesDir: resolveFixturesDir(cfg.FixturesDir),
		result:      result,
		bodies:      map[string][]byte{},
		refs:        map[string]string{},
	}
	if err := run.readAll(ctx); err != nil {
		result.Status = CycleStatusFailed
		return err
	}
	return run.normalize()
}

// readAll reads every fixture file. It reports the first failure, and keeps
// reading the rest, so the audit trail names every file.
func (r *fixtureRun) readAll(ctx context.Context) error {
	var firstErr error
	for _, item := range fixtureFiles {
		if err := ctx.Err(); err != nil {
			r.result.Manifest.Errors = append(r.result.Manifest.Errors, err.Error())
			return err
		}
		firstErr = cmp.Or(firstErr, r.read(item))
	}
	return firstErr
}

// read reads one fixture file and records it as an artifact. A file that cannot
// be read, or that is empty, still produces a failed artifact, so the audit
// trail names it.
func (r *fixtureRun) read(item fixtureFile) error {
	ref := filepath.ToSlash(filepath.Join("data", "examples", item.file))
	body, err := readFixtureBody(filepath.Join(r.fixturesDir, item.file), item.file)
	status := audit.ArtifactStatusOK
	if err != nil {
		status = audit.ArtifactStatusFailed
	}
	r.result.addArtifact(audit.NewArtifact(item.kind, string(config.SourceModeFixtures), ref, body, r.collector.now(), status, err))
	if err != nil {
		r.result.addError(err)
		return err
	}
	r.bodies[item.kind] = body
	r.refs[item.kind] = ref
	return nil
}

// normalize turns the fixture bodies into the normalized cycle.
func (r *fixtureRun) normalize() error {
	normalized, err := normalizeFixtureArtifacts(r.bodies, r.refs, r.result.RawArtifacts, r.cfg, r.result.StartedAt)
	if err != nil {
		r.result.Status = CycleStatusFailed
		r.result.addError(err)
		return err
	}
	r.result.Normalized = normalized
	r.result.Status = CycleStatusOK
	return nil
}

// readFixtureBody reads one fixture file. An empty file is a failure, because a
// report export always carries at least a header row.
func readFixtureBody(path, file string) ([]byte, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return body, err
	}
	if len(body) == 0 {
		return body, fmt.Errorf("%s fixture is empty", file)
	}
	return body, nil
}

func normalizeFixtureArtifacts(bodies map[string][]byte, refs map[string]string, artifacts []audit.RawArtifact, cfg config.Config, collectedAt time.Time) (normalizer.NormalizedCycle, error) {
	submissions, err := normalizer.NormalizeArticles(bytes.NewReader(bodies[artifactKindArticles]), normalizer.ArtifactRef{Kind: artifactKindArticles, PathOrURL: refs[artifactKindArticles]})
	if err != nil {
		return normalizer.NormalizedCycle{}, err
	}
	reviews, err := normalizer.NormalizeReviews(bytes.NewReader(bodies[artifactKindReviews]), normalizer.ArtifactRef{Kind: artifactKindReviews, PathOrURL: refs[artifactKindReviews]})
	if err != nil {
		return normalizer.NormalizedCycle{}, err
	}
	viewMetrics, err := normalizer.NormalizeViews(bytes.NewReader(bodies[artifactKindViews]), normalizer.ArtifactRef{Kind: artifactKindViews, PathOrURL: refs[artifactKindViews]})
	if err != nil {
		return normalizer.NormalizedCycle{}, err
	}
	return normalizer.NormalizedCycle{
		SourceMode:  string(cfg.SourceMode),
		CollectedAt: collectedAt,
		Submissions: submissions,
		Reviews:     reviews,
		ViewMetrics: viewMetrics,
		Artifacts:   artifacts,
	}, nil
}

func resolveFixturesDir(dir string) string {
	if dir == "" {
		dir = "../../data/examples"
	}
	if isDir(dir) {
		return dir
	}
	wd, err := os.Getwd()
	if err != nil {
		return dir
	}
	return searchUpForFixtures(wd, dir)
}

// searchUpForFixtures walks up from start, looking for a data/examples
// directory. It answers with fallback when it reaches the root without a hit.
func searchUpForFixtures(start, fallback string) string {
	for current := start; ; current = filepath.Dir(current) {
		candidate := filepath.Join(current, "data", "examples")
		if isDir(candidate) {
			return candidate
		}
		if filepath.Dir(current) == current {
			return fallback
		}
	}
}

// isDir reports whether a path names an existing directory.
func isDir(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}
