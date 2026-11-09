package monitoring

import (
	"server/internal/collections"
	"server/internal/pbfilter"

	"github.com/pocketbase/pocketbase/core"
)

func loadTracking(app core.App) (map[string]*LocalTracking, error) {
	records, err := app.FindAllRecords(collections.SubmissionTracking)
	if err != nil {
		return nil, err
	}
	items := make(map[string]*LocalTracking, len(records))
	for _, r := range records {
		items[r.GetString("ojs_submission_id")] = trackingView(r)
	}
	return items, nil
}

func trackingView(record *core.Record) *LocalTracking {
	return &LocalTracking{
		ID:              record.Id,
		StatusDetail:    record.GetString("status_detail"),
		EditorNote:      record.GetString("editor_note"),
		LastVerifiedAt:  record.GetString("last_verified_at"),
		VerifiedBy:      record.GetString("verified_by"),
		Observations:    record.GetString("observations"),
		DecisionDetail:  record.GetString("decision_detail"),
		Conference:      record.GetString("conference"),
		Muted:           record.GetBool("muted"),
		MutedDigest:     record.GetString("muted_digest"),
		UpToDateAt:      record.GetString("up_to_date_at"),
		UpToDateDigest:  record.GetString("up_to_date_digest"),
		UpToDateUntil:   record.GetString("up_to_date_until"),
		UpToDateUntilAt: record.GetDateTime("up_to_date_until").Time(),
		Created:         record.GetString("created"),
		Updated:         record.GetString("updated"),
	}
}

// isMuted reports whether a chief editor muted the submission.
func isMuted(tracking *LocalTracking) bool {
	return tracking != nil && tracking.Muted
}

// upToDateDigestOf is the finding digest stamped when a chief editor set the
// submission aside, or an empty string when nobody set it aside.
func upToDateDigestOf(tracking *LocalTracking) string {
	if tracking == nil {
		return ""
	}
	return tracking.UpToDateDigest
}

// loadTrackingFor is loadTracking scoped to the given submission ids, for
// callers that only need the tracking rows of one page of submissions instead
// of the whole table.
func loadTrackingFor(app core.App, ids []string) (map[string]*LocalTracking, error) {
	items := make(map[string]*LocalTracking, len(ids))
	if len(ids) == 0 {
		return items, nil
	}
	filter, params := pbfilter.IDFilter("ojs_submission_id", ids)
	records, err := app.FindRecordsByFilter(collections.SubmissionTracking, filter, "", 0, 0, params)
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		items[r.GetString("ojs_submission_id")] = trackingView(r)
	}
	return items, nil
}
