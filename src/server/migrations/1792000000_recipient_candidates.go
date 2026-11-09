package migrations

import (
	"net/mail"
	"server/internal/normalizer"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// legacyRecipient is a recipients row read under the old kind+key schema,
// captured before the schema and data changes in
// 1792000000_recipient_identity_merge.go run.
type legacyRecipient struct {
	id, kind, key, name, email string
}

// partitionRecipientsByEmail splits recipients into the ones with a real
// e-mail address, read into the legacy shape the merge works with, and the
// ones without, which the merge drops as sync noise.
func partitionRecipientsByEmail(records []*core.Record) ([]legacyRecipient, []*core.Record) {
	valid := make([]legacyRecipient, 0, len(records))
	invalid := make([]*core.Record, 0)
	for _, record := range records {
		email := validRecipientEmail(record.GetString("email"))
		if email == "" {
			invalid = append(invalid, record)
			continue
		}
		valid = append(valid, legacyRecipient{id: record.Id, kind: record.GetString("kind"), key: record.GetString("key"), name: record.GetString(fieldName), email: email})
	}
	return valid, invalid
}

func validRecipientEmail(value string) string {
	value = strings.TrimSpace(value)
	if _, err := mail.ParseAddress(value); err != nil {
		return ""
	}
	return value
}

func deleteRecipients(app core.App, records []*core.Record) error {
	for _, record := range records {
		if err := app.Delete(record); err != nil {
			return err
		}
	}
	return nil
}

func groupRecipientsBySlug(valid []legacyRecipient) map[string][]legacyRecipient {
	groups := map[string][]legacyRecipient{}
	for _, item := range valid {
		slug := normalizer.NameSlug(item.name)
		groups[slug] = append(groups[slug], item)
	}
	return groups
}
