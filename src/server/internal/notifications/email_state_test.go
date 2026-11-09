package notifications_test

import (
	"bytes"
	"context"
	"server/internal/collections"
	"server/internal/notifications"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	mail "server/internal/email"

	_ "server/migrations"
)

// MatchEmailState walks the notifications, targets, requests and attempts one
// rule match leaves behind. The walk used to live in internal/editorial, as
// seven one-caller methods that read these four collections directly. These
// cases moved here with it.
func TestMatchEmailStateFollowsOneMatchFromUnknownToSent(t *testing.T) {
	app, matchID := seedRuleDelivery(t, ruleRN1, []string{testEditorEmail})

	if state := notifications.MatchEmailState(app, matchID); state != notifications.EmailStateUnknown {
		t.Fatalf("before the notification exists: state=%v, want unknown", state)
	}

	if err := newTestService(app).CreateForMatch(matchID); err != nil {
		t.Fatal(err)
	}
	if state := notifications.MatchEmailState(app, matchID); state != notifications.EmailStateUnsent {
		t.Fatalf("after the request is queued: state=%v, want unsent", state)
	}

	runDeliveryWorker(t, app)
	if state := notifications.MatchEmailState(app, matchID); state != notifications.EmailStateSent {
		t.Fatalf("after the e-mail is sent: state=%v, want sent", state)
	}
}

// A match the caller does not know is not an unsent e-mail. It is a broken
// trail, and the caller has to be able to tell the two apart.
func TestMatchEmailStateIsUnknownForAMatchWithNoNotification(t *testing.T) {
	app, _ := seedRuleDelivery(t, ruleRN1, []string{testEditorEmail})
	if state := notifications.MatchEmailState(app, "no-such-match"); state != notifications.EmailStateUnknown {
		t.Fatalf("state=%v, want unknown", state)
	}
}

// A notification whose target carries no delivery request says nothing about
// what was meant to happen, so the answer stays unknown rather than unsent.
func TestMatchEmailStateIsUnknownWhenATargetHasNoRequest(t *testing.T) {
	app, matchID := seedRuleDelivery(t, ruleRN1, []string{testEditorEmail})
	if err := newTestService(app).CreateForMatch(matchID); err != nil {
		t.Fatal(err)
	}
	requests, err := app.FindAllRecords(collections.EmailDeliveryRequests)
	if err != nil || len(requests) != 1 {
		t.Fatalf("requests=%d err=%v", len(requests), err)
	}
	if err := app.Delete(requests[0]); err != nil {
		t.Fatal(err)
	}
	if state := notifications.MatchEmailState(app, matchID); state != notifications.EmailStateUnknown {
		t.Fatalf("state=%v, want unknown", state)
	}
}

func runDeliveryWorker(t *testing.T, app core.App) {
	t.Helper()
	var output bytes.Buffer
	worker := &mail.Worker{App: app, Transport: &mail.MockTransport{Output: &output}, OwnerID: "email-state-worker", From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
}
