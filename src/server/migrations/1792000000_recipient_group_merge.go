package migrations

import (
	"fmt"
	"server/internal/normalizer"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const recipientKindReviewer = "reviewer"

func mergeAllRecipients(app core.App) error {
	all, err := app.FindAllRecords("recipients")
	if err != nil {
		return err
	}
	return mergeRecipientRows(app, all)
}

func mergeRecipientRows(app core.App, all []*core.Record) error {
	valid, invalid := partitionRecipientsByEmail(all)
	if err := deleteRecipients(app, invalid); err != nil {
		return err
	}
	return mergeRecipientGroups(app, groupRecipientsBySlug(valid))
}

func mergeRecipientGroups(app core.App, groups map[string][]legacyRecipient) error {
	for _, group := range groups {
		if err := mergeRecipientGroup(app, group); err != nil {
			return err
		}
	}
	return nil
}

// mergeRecipientGroup resolves one name-slug's worth of legacy rows. A
// reviewer's identity (their username) never merges with another reviewer's,
// even if they share a slug: only editor rows fold into a matching reviewer
// row, or into each other when no reviewer claims the slug.
func mergeRecipientGroup(app core.App, group []legacyRecipient) error {
	reviewers, editors := splitRecipientsByKind(group)
	if len(reviewers) == 0 {
		return mergeIntoSurvivor(app, editors[0], editors[1:])
	}
	if err := mergeIntoSurvivor(app, reviewers[0], editors); err != nil {
		return err
	}
	return finalizeRemainingReviewers(app, reviewers[1:])
}

func splitRecipientsByKind(group []legacyRecipient) (reviewers, editors []legacyRecipient) {
	for _, item := range group {
		if item.kind == recipientKindReviewer {
			reviewers = append(reviewers, item)
		} else {
			editors = append(editors, item)
		}
	}
	return
}

func mergeIntoSurvivor(app core.App, survivor legacyRecipient, duplicates []legacyRecipient) error {
	for _, dup := range duplicates {
		if err := repointRecipientReferences(app, dup.id, survivor.id); err != nil {
			return err
		}
	}
	return finalizeRecipientGroup(app, survivor, recipientHasEditor(survivor, duplicates), survivor.kind == recipientKindReviewer)
}

func recipientHasEditor(survivor legacyRecipient, duplicates []legacyRecipient) bool {
	if survivor.kind == "editor" {
		return true
	}
	return len(duplicates) > 0
}

func finalizeRemainingReviewers(app core.App, reviewers []legacyRecipient) error {
	for _, reviewer := range reviewers {
		if err := finalizeRecipientGroup(app, reviewer, false, true); err != nil {
			return err
		}
	}
	return nil
}

// finalizeRecipientGroup writes the merged identity onto the surviving row:
// a reviewer keeps their username, an editor-only survivor gets the slug.
func finalizeRecipientGroup(app core.App, survivor legacyRecipient, isEditor, isReviewer bool) error {
	record, err := app.FindRecordById("recipients", survivor.id)
	if err != nil {
		return err
	}
	slug := normalizer.NameSlug(survivor.name)
	identity := slug
	if survivor.kind == recipientKindReviewer {
		identity = survivor.key
	}
	record.Set("identity", identity)
	record.Set("name_slug", slug)
	record.Set("is_editor", isEditor)
	record.Set("is_reviewer", isReviewer)
	return app.Save(record)
}

// repointRecipientReferences moves every relation pointing at a duplicate
// recipient row onto the surviving row, then deletes the duplicate.
// PocketBase relation fields do not cascade-delete by default, and neither
// notification_targets.recipient nor email_delivery_attempts.recipient is
// configured to, so deleting the duplicate first would leave those rows
// pointing at a missing recipient.
func repointRecipientReferences(app core.App, fromID, toID string) error {
	for _, table := range []string{"notification_targets", "email_delivery_attempts"} {
		query := fmt.Sprintf("UPDATE %s SET recipient = {:to} WHERE recipient = {:from}", table)
		if _, err := app.DB().NewQuery(query).Bind(dbx.Params{"to": toID, "from": fromID}).Execute(); err != nil {
			return err
		}
	}
	_, err := app.DB().NewQuery("DELETE FROM recipients WHERE id = {:id}").Bind(dbx.Params{"id": fromID}).Execute()
	return err
}
