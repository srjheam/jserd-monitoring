package monitoring

import (
	"encoding/json"
	"fmt"
	"server/internal/collections"
	"server/internal/editorial"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// effectiveState is every derived column of a materialized row.
type effectiveState struct {
	finalState     string
	queue          string
	attention      string
	priority       int
	digest         string
	muted          bool
	upToDateDigest string
}

// deriveEffective computes every derived column from the four inputs that
// decide them. This is the one implementation. applyEffective compares it
// against a stored row, storeSubmission writes it to a new row, and
// acceptedSubmissions copies three of its fields into a projected item, so the
// stored row and the projection can never answer differently.
func deriveEffective(status string, copyeditorAssigned *bool, tracking *LocalTracking, findings []editorial.Finding) effectiveState {
	finalState := effectiveFinal(status)
	return effectiveState{
		finalState:     finalState,
		queue:          queueFor(status, finalState, copyeditorAssigned, tracking, findings),
		attention:      editorial.Rank(findings),
		priority:       editorial.PriorityFor(findings),
		digest:         editorial.DigestFor(findings),
		muted:          isMuted(tracking),
		upToDateDigest: upToDateDigestOf(tracking),
	}
}

// writeEffective sets the seven derived columns on a record. Adding a column to
// effectiveState and to this function is what makes every writer carry it.
func writeEffective(record *core.Record, state effectiveState) {
	record.Set("effective_final_state", state.finalState)
	record.Set("effective_queue", state.queue)
	record.Set("effective_attention", state.attention)
	record.Set("effective_priority", state.priority)
	record.Set("finding_digest", state.digest)
	record.Set("muted", state.muted)
	record.Set("up_to_date_digest", state.upToDateDigest)
}

// storedEffective reads the seven derived columns back off a record, so
// applyEffective can tell an unchanged row from one that needs a save.
func storedEffective(record *core.Record) effectiveState {
	return effectiveState{
		finalState:     record.GetString("effective_final_state"),
		queue:          record.GetString("effective_queue"),
		attention:      record.GetString("effective_attention"),
		priority:       record.GetInt("effective_priority"),
		digest:         record.GetString("finding_digest"),
		muted:          record.GetBool("muted"),
		upToDateDigest: record.GetString("up_to_date_digest"),
	}
}

// recomputeEffective refreshes the stored effective_* columns of one
// materialized row after its local tracking changed.
func recomputeEffective(app core.App, ojsSubmissionID string) error {
	rows, err := app.FindRecordsByFilter(collections.OJSSubmissions, "ojs_submission_id = {:id}", "", 1, 0, dbx.Params{"id": ojsSubmissionID})
	if err != nil || len(rows) == 0 {
		return err
	}
	tracking, findings, err := effectiveInputs(app, ojsSubmissionID)
	if err != nil {
		return err
	}
	return applyEffective(app, rows[0], tracking, findings)
}

// effectiveInputs is the curation and the findings that together decide the
// derived state of one submission.
func effectiveInputs(app core.App, ojsSubmissionID string) (*LocalTracking, []editorial.Finding, error) {
	tracking, err := submissionTracking(app, ojsSubmissionID)
	if err != nil {
		return nil, nil, err
	}
	findings, err := latestFindingsFor(app, []string{ojsSubmissionID})
	if err != nil {
		return nil, nil, err
	}
	return tracking, findings[ojsSubmissionID], nil
}

// recomputeAll refreshes the stored effective_* columns of every materialized
// row after a new editorial run replaced the findings.
func recomputeAll(app core.App) error {
	rows, err := app.FindAllRecords(collections.OJSSubmissions)
	if err != nil || len(rows) == 0 {
		return err
	}
	tracking, findings, err := allEffectiveInputs(app)
	if err != nil {
		return err
	}
	return applyEffectiveToAll(app, rows, tracking, findings)
}

// allEffectiveInputs is the curation and the findings of every submission.
func allEffectiveInputs(app core.App) (map[string]*LocalTracking, map[string][]editorial.Finding, error) {
	tracking, err := loadTracking(app)
	if err != nil {
		return nil, nil, err
	}
	findings, err := editorial.LatestFindings(app)
	if err != nil {
		return nil, nil, err
	}
	return tracking, findings, nil
}

// applyEffectiveToAll refreshes the derived columns of every given row.
func applyEffectiveToAll(app core.App, rows []*core.Record, tracking map[string]*LocalTracking, findings map[string][]editorial.Finding) error {
	for _, row := range rows {
		id := row.GetString("ojs_submission_id")
		if err := applyEffective(app, row, tracking[id], findings[id]); err != nil {
			return err
		}
	}
	return nil
}

func applyEffective(app core.App, row *core.Record, tracking *LocalTracking, findings []editorial.Finding) error {
	ojsSubmissionID := row.GetString("ojs_submission_id")
	var evidence Evidence
	if err := json.Unmarshal([]byte(row.GetString("evidence")), &evidence); err != nil {
		return fmt.Errorf("decode evidence of %s: %w", ojsSubmissionID, err)
	}
	state := deriveEffective(row.GetString(statusFieldName), evidence.CopyeditorAssigned, tracking, findings)
	if state == storedEffective(row) {
		return nil
	}
	writeEffective(row, state)
	return app.Save(row)
}
