package monitoring

import (
	"encoding/json"
	"fmt"
	"server/internal/editorial"
	"server/internal/normalizer"
	"server/internal/snapshotpayload"

	"github.com/pocketbase/pocketbase/core"
)

// Evidence holds the submission fields that do not live in dedicated
// ojs_submissions columns. It also keeps observed copyeditor assignment
// evidence available when derived state is recomputed.
// Findings are intentionally excluded: they belong to the editorial pipeline
// and are attached live at query time so newly published runs surface
// without waiting for the next collection cycle.
type Evidence struct {
	URL           string `json:"url"`
	DateSubmitted string `json:"dateSubmitted"`
	LastModified  string `json:"lastModified"`
	// DateLastActivity is OJS's own last-activity timestamp. LastModified
	// above comes from the article export and moves for unrelated edits.
	DateLastActivity   string     `json:"dateLastActivity"`
	Editors            []string   `json:"editors"`
	Decisions          []Decision `json:"decisions"`
	CopyeditorAssigned *bool      `json:"copyeditorAssigned,omitempty"`
	Reviews            []Review   `json:"reviews"`
	Provenance         Provenance `json:"provenance"`
}

// Rebuild replaces the materialized submissions with the projection of the
// latest succeeded snapshot. It is idempotent and safe to run repeatedly.
func Rebuild(app core.App) error {
	cycle, normalized, err := latestNormalizedCycle(app)
	if err != nil {
		return err
	}
	if normalized == nil {
		return clearSubmissions(app)
	}
	items, findings, err := projectedSubmissions(app, *normalized)
	if err != nil {
		return err
	}
	return replaceSubmissions(app, cycle.Id, items, findings)
}

// latestNormalizedCycle is the latest succeeded collection cycle and the
// normalized snapshot it stored. A nil snapshot means that there is nothing to
// project yet.
func latestNormalizedCycle(app core.App) (*core.Record, *normalizer.NormalizedCycle, error) {
	cycle, snapshot, err := latestSuccess(app)
	if err != nil || cycle == nil {
		return nil, nil, err
	}
	normalized, err := snapshotCycle(app, snapshot)
	return cycle, normalized, err
}

// snapshotCycle decodes the normalized cycle a snapshot holds. A nil snapshot
// yields a nil cycle.
func snapshotCycle(app core.App, snapshot *core.Record) (*normalizer.NormalizedCycle, error) {
	if snapshot == nil {
		return nil, nil
	}
	payload, err := snapshotpayload.Read(app, snapshot)
	if err != nil {
		return nil, fmt.Errorf("read latest snapshot: %w", err)
	}
	var normalized normalizer.NormalizedCycle
	if err := json.Unmarshal(payload, &normalized); err != nil {
		return nil, fmt.Errorf("decode latest snapshot: %w", err)
	}
	return &normalized, nil
}

// projectedSubmissions builds the display view of every submission in the
// cycle, together with the findings the editorial pipeline computed.
func projectedSubmissions(app core.App, normalized normalizer.NormalizedCycle) ([]Submission, map[string][]editorial.Finding, error) {
	tracking, findings, err := allEffectiveInputs(app)
	if err != nil {
		return nil, nil, err
	}
	return acceptedSubmissions(normalized, tracking, findings), findings, nil
}

// acceptedSubmissions projects every submission OJS finished accepting. OJS has
// not finished accepting a submission with a non-zero submissionProgress, so it
// is not a real submission for editorial purposes and must not reach the
// dashboard.
func acceptedSubmissions(normalized normalizer.NormalizedCycle, tracking map[string]*LocalTracking, findings map[string][]editorial.Finding) []Submission {
	items := make([]Submission, 0, len(normalized.Submissions))
	for _, source := range normalized.Submissions {
		if source.SubmissionProgress != 0 {
			continue
		}
		reviews := normalized.Reviews
		itemTracking := tracking[source.OJSSubmissionID]
		itemFindings := findings[source.OJSSubmissionID]
		item := Submission{OJSSubmissionID: source.OJSSubmissionID, Title: source.Title, Status: source.Status, URL: source.URL, DateSubmitted: source.DateSubmitted, LastModified: source.LastModified, DateLastActivity: source.DateLastActivity, CopyeditorAssigned: source.CopyeditorAssigned, Editors: []string{}, Decisions: []Decision{}, Reviews: []Review{}, Findings: findingsView(itemFindings), Tracking: itemTracking, Provenance: Provenance{"ojsSubmissionId": availability(source.OJSSubmissionID), "title": availability(source.Title), statusFieldName: availability(source.Status), "url": availability(source.URL), "dateSubmitted": availability(source.DateSubmitted), "lastModified": availability(source.LastModified), "dateLastActivity": availability(source.DateLastActivity), "editors": unavailableProvenance, "decisions": unavailableProvenance, "reviews": unavailableProvenance, "findings": computedProvenance, "tracking": unavailableProvenance, "effectiveFinalState": computedProvenance, "effectiveQueue": computedProvenance, "effectiveAttention": computedProvenance}}
		markCuratedProvenance(item.Provenance, itemTracking)
		addEditorsAndDecisions(&item, source.Editors)
		addReviews(&item, source.OJSSubmissionID, reviews)
		state := deriveEffective(source.Status, item.CopyeditorAssigned, itemTracking, itemFindings)
		item.EffectiveFinalState = state.finalState
		item.EffectiveQueue = state.queue
		item.EffectiveAttention = state.attention
		items = append(items, item)
	}
	return items
}
