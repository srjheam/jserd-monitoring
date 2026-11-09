package state

import (
	"fmt"
	"server/internal/collections"
	"server/internal/normalizer"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func upsertRecipients(app core.App, collection *core.Collection, items []recipientObservation) error {
	for _, item := range items {
		if item.email == "" {
			continue
		}
		if err := upsertRecipient(app, collection, item); err != nil {
			return err
		}
	}
	return nil
}

func candidateIdentity(item recipientObservation) string {
	if item.isReviewer {
		return item.username
	}
	return normalizer.NameSlug(item.name)
}

func upsertRecipient(app core.App, collection *core.Collection, item recipientObservation) error {
	record, err := findRecipient(app, item)
	if err != nil {
		return err
	}
	if record == nil {
		record = core.NewRecord(collection)
		record.Set("identity", candidateIdentity(item))
	}
	applyRecipientObservation(record, item)
	if err := app.Save(record); err != nil {
		return fmt.Errorf("sync recipient: %w", err)
	}
	return nil
}

func applyRecipientObservation(record *core.Record, item recipientObservation) {
	if item.isReviewer {
		record.Set("is_reviewer", true)
		if record.GetString("identity") != item.username {
			record.Set("identity", item.username)
		}
	} else {
		record.Set("is_editor", true)
	}
	record.Set("name", item.name)
	record.Set("email", item.email)
	record.Set("name_slug", normalizer.NameSlug(item.name))
}

// findRecipient implements the ADR-0015 lookup order: identity match, then
// name-slug match (the cross-role merge), then e-mail match (the rename
// fallback). The first hit wins; no match means a new recipient.
func findRecipient(app core.App, item recipientObservation) (*core.Record, error) {
	lookups := []func() (*core.Record, error){
		func() (*core.Record, error) { return findRecipientBy(app, "identity", candidateIdentity(item)) },
		func() (*core.Record, error) { return findRecipientBy(app, "name_slug", normalizer.NameSlug(item.name)) },
		func() (*core.Record, error) { return findRecipientBy(app, "email", item.email) },
	}
	for _, lookup := range lookups {
		record, err := lookup()
		if err != nil {
			return nil, err
		}
		if record != nil {
			return record, nil
		}
	}
	return nil, nil
}

func findRecipientBy(app core.App, field, value string) (*core.Record, error) {
	if value == "" {
		return nil, nil
	}
	records, err := app.FindRecordsByFilter(collections.Recipients, field+" = {:v}", "", 1, 0, dbx.Params{"v": value})
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return records[0], nil
}
