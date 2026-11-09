package notifications_test

import (
	"context"
	"server/internal/collections"
	mail "server/internal/email"
	"server/internal/events"
	"testing"
)

func TestSpecificRecipientRoleIsCopiedToTargetsAttemptsAndWelcome(t *testing.T) {
	app := notificationsTestApp(t)
	seedWelcomeRecipient(t, app, testEditorKey, testEditorEmail)
	matchID := seedWelcomeMatch(t, app, events.RuleRN18, "role-history", []string{testEditorKey})
	if err := newTestService(app).CreateForMatch(matchID); err != nil {
		t.Fatal(err)
	}

	targets, err := app.FindAllRecords(collections.NotificationTargets)
	if err != nil || len(targets) != 2 {
		t.Fatalf("targets=%d err=%v, want notification and welcome", len(targets), err)
	}
	for _, target := range targets {
		if role := target.GetString("recipient_role"); role != events.RecipientRoleJournalEditor {
			t.Errorf("target role=%q, want %q", role, events.RecipientRoleJournalEditor)
		}
	}

	worker := &mail.Worker{App: app, Transport: &mail.MockTransport{}, OwnerID: "role-history", From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if err != nil || len(attempts) != 2 {
		t.Fatalf("attempts=%d err=%v, want notification and welcome", len(attempts), err)
	}
	for _, attempt := range attempts {
		if role := attempt.GetString("recipient_role"); role != events.RecipientRoleJournalEditor {
			t.Errorf("attempt role=%q, want %q", role, events.RecipientRoleJournalEditor)
		}
	}
}

func TestAssociateEditorRoleIsCopiedFromFilteredNotificationTargetToAttempt(t *testing.T) {
	app, matchID := seedRuleDelivery(t, ruleRN1, []string{testEditorEmail})
	if err := newTestService(app).CreateForMatch(matchID); err != nil {
		t.Fatal(err)
	}
	targets, err := app.FindAllRecords(collections.NotificationTargets)
	if err != nil || len(targets) != 1 {
		t.Fatalf("targets=%d err=%v, want one", len(targets), err)
	}
	if got := targets[0].GetString("recipient_role"); got != events.RecipientRoleAssociateEditor {
		t.Fatalf("target role=%q, want associate editor", got)
	}

	worker := &mail.Worker{App: app, Transport: &mail.MockTransport{}, OwnerID: "associate-role", From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("attempts=%d err=%v, want one", len(attempts), err)
	}
	if got := attempts[0].GetString("recipient_role"); got != events.RecipientRoleAssociateEditor {
		t.Fatalf("attempt role=%q, want associate editor", got)
	}
}
