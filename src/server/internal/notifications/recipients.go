package notifications

import (
	"server/internal/collections"
	"server/internal/events"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// resolvedRecipient pairs a matched recipients row with the key that found
// it: an editor's name-slug or a reviewer's username. This is not always the
// row's current identity, since a dual-role recipient's identity can be
// promoted to a username after an editor-target lookup already matched it
// by slug.
type resolvedRecipient struct {
	Record    *core.Record
	TargetKey string
}

func resolveRecipients(app core.App, rule, targetKey string, editorTargetKeys []string) ([]resolvedRecipient, string, error) {
	if events.IsReviewerRule(rule) {
		recipients, err := resolveReviewerRecipient(app, targetKey)
		return recipients, "reviewer", err
	}
	if len(editorTargetKeys) == 0 {
		return nil, "editor", nil
	}
	recipients, err := resolveEditorRecipients(app, editorTargetKeys)
	return recipients, "editor", err
}

func resolveReviewerRecipient(app core.App, targetKey string) ([]resolvedRecipient, error) {
	records, err := app.FindRecordsByFilter(collections.Recipients, "identity = {:key} && is_reviewer = true", "", 1, 0, dbx.Params{"key": targetKey})
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return []resolvedRecipient{{Record: records[0], TargetKey: targetKey}}, nil
}

func resolveEditorRecipients(app core.App, editorTargetKeys []string) ([]resolvedRecipient, error) {
	result := []resolvedRecipient{}
	for _, key := range editorTargetKeys {
		record, err := findEditorRecipient(app, key)
		if err != nil {
			return nil, err
		}
		if record != nil {
			result = append(result, resolvedRecipient{Record: record, TargetKey: key})
		}
	}
	return result, nil
}

func findEditorRecipient(app core.App, key string) (*core.Record, error) {
	records, err := app.FindRecordsByFilter(collections.Recipients, "name_slug = {:key} && is_editor = true", "", 1, 0, dbx.Params{"key": key})
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return records[0], nil
}
