package monitoring

import (
	"fmt"
	"server/internal/audit"
	"server/internal/collections"
	"server/internal/editorial"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// RegisterHooks keeps the materialized ojs_submissions rows aligned with the
// latest succeeded collection cycle and with local tracking edits.
func RegisterHooks(app core.App) {
	app.OnRecordAfterCreateSuccess(collections.CollectionCycles).BindFunc(rebuildAfterCycle)
	app.OnRecordAfterUpdateSuccess(collections.CollectionCycles).BindFunc(rebuildAfterCycle)

	app.OnRecordAfterCreateSuccess(collections.SubmissionTracking).BindFunc(recomputeAfterCuration)
	app.OnRecordAfterUpdateSuccess(collections.SubmissionTracking).BindFunc(recomputeAfterCuration)
	app.OnRecordAfterDeleteSuccess(collections.SubmissionTracking).BindFunc(recomputeAfterCuration)

	app.OnRecordCreate(collections.SubmissionTracking).BindFunc(stampMutedDigest)
	app.OnRecordUpdate(collections.SubmissionTracking).BindFunc(stampMutedDigest)
}

// rebuildAfterCycle rebuilds the projection once a collection cycle succeeds.
func rebuildAfterCycle(event *core.RecordEvent) error {
	if event.Record.GetString(statusFieldName) == "succeeded" {
		if err := Rebuild(event.App); err != nil {
			event.App.Logger().Error("monitoring materialization failed", "error", audit.RedactError(err.Error()))
		}
	}
	return event.Next()
}

// recomputeAfterCuration refreshes the derived state of one submission after a
// chief editor curated it.
func recomputeAfterCuration(event *core.RecordEvent) error {
	submissionID := event.Record.GetString("ojs_submission_id")
	if submissionID == "" {
		return event.Next()
	}
	if err := recomputeEffective(event.App, submissionID); err != nil {
		event.App.Logger().Error("monitoring effective state update failed", "submission", submissionID, "error", audit.RedactError(err.Error()))
	}
	return event.Next()
}

// stampMutedDigest records the finding digest at the moment of the mute, and
// the Muted screen reads it to mark a finding that appeared afterwards. It is
// stamped here rather than by the client because the digest must describe the
// findings the server holds now, not the ones the open page was rendered from.
// An empty stored value is the only "not stamped" state: a submission with no
// findings still hashes to a non-empty digest.
func stampMutedDigest(event *core.RecordEvent) error {
	if !event.Record.GetBool("muted") {
		// Unmuting clears the stamp, so a later mute of the same
		// submission stamps the findings of that later moment.
		event.Record.Set("muted_digest", "")
		return event.Next()
	}
	if event.Record.GetString("muted_digest") == "" {
		submissionID := event.Record.GetString("ojs_submission_id")
		findings, err := latestFindingsFor(event.App, []string{submissionID})
		if err != nil {
			return err
		}
		event.Record.Set("muted_digest", editorial.DigestFor(findings[submissionID]))
	}
	return event.Next()
}

// OnFindingsReady refreshes derived attention state once a cycle replaces the
// findings of every submission. It is the coordinator's Coordinator.OnFindingsReady
// hook, called synchronously right after Store.ReplaceFindings succeeds. It
// also doubles as the recurring task that expires a passed up_to_date_until:
// without it, a return date only takes effect on the next write to that
// tracking row.
func OnFindingsReady(app core.App, _ string) error {
	if err := clearExpiredUpToDate(app); err != nil {
		return fmt.Errorf("monitoring up to date expiry: %w", err)
	}
	if err := recomputeAll(app); err != nil {
		return fmt.Errorf("monitoring attention refresh: %w", err)
	}
	return nil
}

// clearExpiredUpToDate clears the up_to_date_at, up_to_date_digest and
// up_to_date_until of every tracking record whose return date has passed.
func clearExpiredUpToDate(app core.App) error {
	records, err := app.FindAllRecords(collections.SubmissionTracking)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, record := range records {
		if err := clearUpToDateIfPassed(app, record, now); err != nil {
			return err
		}
	}
	return nil
}

// clearUpToDateIfPassed clears the up-to-date mark of one tracking record once
// its return date passed. A record with no return date, or with one still
// ahead, keeps its mark.
func clearUpToDateIfPassed(app core.App, record *core.Record, now time.Time) error {
	until := record.GetDateTime("up_to_date_until")
	if until.IsZero() || until.Time().After(now) {
		return nil
	}
	record.Set("up_to_date_at", "")
	record.Set("up_to_date_digest", "")
	record.Set("up_to_date_until", "")
	return app.Save(record)
}
