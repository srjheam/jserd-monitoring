package state

import (
	"fmt"
	"server/internal/collections"
	"server/internal/dberr"
	"server/internal/events"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func (s *Store) saveMatch(app core.App, eventID string, match events.RuleMatch) error {
	existing, err := findMatch(app, eventID, match.RuleCode)
	if err != nil || existing != nil {
		return err
	}
	return s.insertMatch(app, eventID, match)
}

// insertMatch writes one new rule match row.
func (s *Store) insertMatch(app core.App, eventID string, match events.RuleMatch) error {
	collection, err := app.FindCollectionByNameOrId(collections.EventRuleMatches)
	if err != nil {
		return fmt.Errorf("find event rule matches collection: %w", err)
	}
	record := core.NewRecord(collection)
	record.Set("event", eventID)
	record.Set("rule_code", match.RuleCode)
	record.Set("status", match.Status)
	if err := s.saveRecord(app, record); err != nil {
		return s.resolveMatchConflict(app, eventID, match.RuleCode, err)
	}
	return nil
}

// resolveMatchConflict accepts the match another writer stored first. A failure
// that is not a unique-key conflict is reported as it is.
func (s *Store) resolveMatchConflict(app core.App, eventID, ruleCode string, saveErr error) error {
	if !dberr.IsConstraint(saveErr) {
		return fmt.Errorf("save event rule match: %w", saveErr)
	}
	existing, err := findMatch(app, eventID, ruleCode)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("save match constraint without matching event and rule: %w", saveErr)
	}
	return nil
}

func findMatch(app core.App, eventID, ruleCode string) (*core.Record, error) {
	records, err := app.FindRecordsByFilter(collections.EventRuleMatches, "event = {:event} && rule_code = {:rule}", "", 1, 0, dbx.Params{"event": eventID, "rule": ruleCode})
	if err != nil {
		return nil, fmt.Errorf("find event rule match: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}
