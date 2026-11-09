package monitoring

import (
	"encoding/json"
	"fmt"
	"server/internal/collections"
	"server/internal/editorial"

	"github.com/pocketbase/pocketbase/core"
)

func clearSubmissions(app core.App) error {
	if _, err := app.DB().NewQuery("DELETE FROM ojs_submissions").Execute(); err != nil {
		return fmt.Errorf("clear ojs_submissions: %w", err)
	}
	return nil
}

func replaceSubmissions(app core.App, cycleID string, items []Submission, findings map[string][]editorial.Finding) error {
	collection, err := app.FindCollectionByNameOrId(collections.OJSSubmissions)
	if err != nil {
		return err
	}
	return app.RunInTransaction(func(tx core.App) error {
		return storeSubmissions(tx, collection, cycleID, items, findings)
	})
}

// storeSubmissions replaces every materialized row with the given projection.
func storeSubmissions(tx core.App, collection *core.Collection, cycleID string, items []Submission, findings map[string][]editorial.Finding) error {
	if err := clearSubmissions(tx); err != nil {
		return err
	}
	for _, item := range items {
		if err := storeSubmission(tx, collection, cycleID, item, findings[item.OJSSubmissionID]); err != nil {
			return err
		}
	}
	return nil
}

// storeSubmission writes one materialized row.
func storeSubmission(txApp core.App, collection *core.Collection, cycleID string, item Submission, findings []editorial.Finding) error {
	evidence, err := evidenceFields(item)
	if err != nil {
		return err
	}
	record := core.NewRecord(collection)
	record.Set("ojs_submission_id", item.OJSSubmissionID)
	record.Set("title", item.Title)
	record.Set(statusFieldName, item.Status)
	writeEffective(record, deriveEffective(item.Status, item.CopyeditorAssigned, item.Tracking, findings))
	record.Set("cycle", cycleID)
	record.Set("evidence", evidence)
	if err := txApp.Save(record); err != nil {
		return fmt.Errorf("store submission %s: %w", item.OJSSubmissionID, err)
	}
	return nil
}

// evidenceFields encodes submission evidence, including Copyeditor assignment,
// in the shape PocketBase stores in its JSON field.
func evidenceFields(item Submission) (map[string]any, error) {
	evidence, err := json.Marshal(Evidence{
		URL:                item.URL,
		DateSubmitted:      item.DateSubmitted,
		LastModified:       item.LastModified,
		DateLastActivity:   item.DateLastActivity,
		Editors:            item.Editors,
		Decisions:          item.Decisions,
		CopyeditorAssigned: item.CopyeditorAssigned,
		Reviews:            item.Reviews,
		Provenance:         item.Provenance,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal evidence for %s: %w", item.OJSSubmissionID, err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(evidence, &decoded); err != nil {
		return nil, fmt.Errorf("decode evidence for %s: %w", item.OJSSubmissionID, err)
	}
	return decoded, nil
}
