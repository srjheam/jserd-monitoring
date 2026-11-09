package notifications

import (
	"errors"
	"fmt"
	"server/internal/collections"
	"server/internal/dberr"
	"server/internal/events"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type Service struct {
	App           core.App
	EditorTargets EditorTargets
}

func NewService(app core.App) *Service { return &Service{App: app} }

func (s *Service) CreatePending() error {
	matches, err := s.App.FindRecordsByFilter(collections.EventRuleMatches, "", "id", 0, 0)
	if err != nil {
		return fmt.Errorf("find pending matches: %w", err)
	}
	var errs []error
	for _, match := range matches {
		if err := s.createPendingFor(match.Id); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// createPendingFor records the notification one rule match calls for. A match
// that already has a notification calls for nothing.
func (s *Service) createPendingFor(matchID string) error {
	existing, err := s.notificationFor(s.App, matchID)
	if err != nil {
		return fmt.Errorf("check match %s: %w", matchID, err)
	}
	if existing {
		return nil
	}
	if err := s.CreateForMatch(matchID); err != nil {
		return fmt.Errorf("create notification for match %s: %w", matchID, err)
	}
	return nil
}

// notificationFor reports whether a rule match already has a notification.
func (s *Service) notificationFor(app core.App, matchID string) (bool, error) {
	existing, err := app.FindRecordsByFilter(collections.Notifications, "rule_match = {:id}", "", 1, 0, dbx.Params{"id": matchID})
	if err != nil {
		return false, err
	}
	return len(existing) > 0, nil
}

func (s *Service) CreateForMatch(matchID string) error {
	existing, err := s.notificationFor(s.App, matchID)
	if err != nil || existing {
		return err
	}
	editorKeys, err := s.editorKeysForMatch(matchID)
	if err != nil {
		return err
	}
	err = s.App.RunInTransaction(func(app core.App) error {
		return s.createNotification(app, matchID, editorKeys)
	})
	if err == nil || !dberr.IsConstraint(err) {
		return err
	}
	return s.acceptExistingNotification(matchID, err)
}

// acceptExistingNotification accepts the notification another writer created
// first. A unique-key conflict with no matching row left is a real failure.
func (s *Service) acceptExistingNotification(matchID string, saveErr error) error {
	existing, findErr := s.notificationFor(s.App, matchID)
	if findErr != nil || !existing {
		return fmt.Errorf("notification constraint without matching rule_match: %w", saveErr)
	}
	return nil
}

// createNotification records one notification and its delivery requests.
func (s *Service) createNotification(app core.App, matchID string, editorKeys []string) error {
	match, rule, err := supportedMatch(app, matchID)
	if err != nil {
		return err
	}
	existing, err := s.notificationFor(app, matchID)
	if err != nil || existing {
		return err
	}
	return s.notifyRecipients(app, match, rule, editorKeys)
}

// supportedMatch reads one rule match and the rule it names. A rule this
// system sends no e-mail for is refused.
func supportedMatch(app core.App, matchID string) (*core.Record, string, error) {
	match, err := app.FindRecordById(collections.EventRuleMatches, matchID)
	if err != nil {
		return nil, "", fmt.Errorf("find match: %w", err)
	}
	rule := match.GetString("rule_code")
	if !events.IsKnownRule(rule) {
		return nil, "", fmt.Errorf("unsupported rule %q", rule)
	}
	return match, rule, nil
}
