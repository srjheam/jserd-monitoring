package notifications

import (
	"server/internal/collections"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// EmailState is what the delivery trail behind one rule match says about the
// e-mails that match called for.
type EmailState int

const (
	// EmailStateSent means that every e-mail the match called for was sent.
	EmailStateSent EmailState = iota
	// EmailStateUnsent means that at least one e-mail is still unsent.
	EmailStateUnsent
	// EmailStateUnknown means that the record trail is broken or unreadable.
	EmailStateUnknown
)

// MatchEmailState walks the trail one rule match leaves, from its notifications
// through their targets and each target's delivery request to that request's
// attempts, and reports whether every e-mail reached its recipient.
//
// The walk lives here because all four collections it reads belong to this
// package. A caller outside it cannot know which rows still count, so a caller
// that read the tables itself would not learn when that answer changes.
// email_state_walk.go holds the two steps below the notification.
func MatchEmailState(app core.App, matchID string) EmailState {
	sent, err := app.FindRecordsByFilter(collections.Notifications, "rule_match = {:id}", "", 0, 0, dbx.Params{"id": matchID})
	if err != nil || len(sent) == 0 {
		return EmailStateUnknown
	}
	for _, notification := range sent {
		if state := notificationEmailState(app, notification.Id); state != EmailStateSent {
			return state
		}
	}
	return EmailStateSent
}
