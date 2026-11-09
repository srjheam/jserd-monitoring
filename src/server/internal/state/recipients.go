package state

import (
	"net/mail"
	"server/internal/collections"
	"server/internal/normalizer"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// recipientObservation is a single editor or reviewer sighting from one
// collection cycle, before it is merged into a recipients row. See ADR 0015.
type recipientObservation struct {
	name, email, username string
	isReviewer            bool
}

func editorObservations(cycle normalizer.NormalizedCycle) []recipientObservation {
	items := []recipientObservation{}
	for _, submission := range cycle.Submissions {
		for _, editor := range submission.Editors {
			items = append(items, recipientObservation{
				name:  strings.TrimSpace(editor.GivenName + " " + editor.FamilyName),
				email: validEmailOrEmpty(editor.Email),
			})
		}
	}
	return items
}

func reviewerObservations(cycle normalizer.NormalizedCycle) []recipientObservation {
	items := []recipientObservation{}
	for _, review := range cycle.Reviews {
		if review.ReviewerUsername == "" {
			continue
		}
		items = append(items, recipientObservation{
			name:       strings.TrimSpace(review.ReviewerGivenName + " " + review.ReviewerFamilyName),
			email:      validEmailOrEmpty(review.ReviewerEmail),
			username:   review.ReviewerUsername,
			isReviewer: true,
		})
	}
	return items
}

func syncRecipients(app core.App, cycle normalizer.NormalizedCycle) error {
	collection, err := app.FindCollectionByNameOrId(collections.Recipients)
	if err != nil {
		return err
	}
	return upsertRecipients(app, collection, append(editorObservations(cycle), reviewerObservations(cycle)...))
}

func validEmailOrEmpty(value string) string {
	value = strings.TrimSpace(value)
	if _, err := mail.ParseAddress(value); err != nil {
		return ""
	}
	return value
}
