package notifications

import (
	"server/internal/collections"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// This file reads the trail below one notification: its targets, each target's
// delivery request, and that request's attempts. email_state.go holds the
// question a caller asks, MatchEmailState, and the EmailState it answers with.

// notificationEmailState is EmailStateSent only when every target of one
// notification received the e-mail.
func notificationEmailState(app core.App, notificationID string) EmailState {
	targets, err := app.FindRecordsByFilter(collections.NotificationTargets, "notification = {:id}", "", 0, 0, dbx.Params{"id": notificationID})
	if err != nil || len(targets) == 0 {
		return EmailStateUnknown
	}
	for _, target := range targets {
		if state := targetEmailState(app, target.Id); state != EmailStateSent {
			return state
		}
	}
	return EmailStateSent
}

// targetEmailState reports whether the e-mail to one recipient was sent. A
// target with no delivery request leaves the answer unknown, because nothing
// says what was meant to happen. A request whose attempts hold no sent one is
// unsent, because the trail is complete and says so.
func targetEmailState(app core.App, targetID string) EmailState {
	requests, err := app.FindRecordsByFilter(collections.EmailDeliveryRequests, "target = {:id}", "", 1, 0, dbx.Params{"id": targetID})
	if err != nil || len(requests) == 0 {
		return EmailStateUnknown
	}
	attempts, err := app.FindRecordsByFilter(collections.EmailDeliveryAttempts, "request = {:id} && status = 'sent'", "", 1, 0, dbx.Params{"id": requests[0].Id})
	if err != nil {
		return EmailStateUnknown
	}
	if len(attempts) == 0 {
		return EmailStateUnsent
	}
	return EmailStateSent
}
