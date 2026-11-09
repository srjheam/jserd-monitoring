package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(createOperationalCollections, dropOperationalCollections)
}

// createOperationalCollections creates the four collections one collection
// cycle writes: the cycle, its snapshot, the events the snapshot implies, and
// the rule matches on those events.
func createOperationalCollections(app core.App) error {
	cycles := newCyclesCollection()
	if err := app.Save(cycles); err != nil {
		return err
	}
	snapshots, err := saveSnapshotsCollection(app, cycles)
	if err != nil {
		return err
	}
	events := newEventsCollection(snapshots)
	if err := app.Save(events); err != nil {
		return err
	}
	return app.Save(newMatchesCollection(events))
}

func dropOperationalCollections(app core.App) error {
	return dropCollections(app, "event_rule_matches", "events", "snapshots", "collection_cycles")
}

func newCyclesCollection() *core.Collection {
	cycles := core.NewBaseCollection("collection_cycles")
	cycles.Fields.Add(
		&core.SelectField{Name: "source_mode", Values: []string{sourceModeFixtures, "live"}, Required: true},
		&core.SelectField{Name: fieldStatus, Values: []string{"succeeded", statusValueFailed}, Required: true},
		&core.DateField{Name: "started_at", Required: true},
		&core.DateField{Name: "finished_at", Required: true},
		&core.JSONField{Name: "manifest", Required: true},
		&core.TextField{Name: "error"},
	)
	cycles.Indexes = append(cycles.Indexes, "CREATE INDEX `idx_collection_cycles_finished_at` ON `collection_cycles` (`finished_at`)")
	return cycles
}

// saveSnapshotsCollection creates the snapshots collection. It saves twice
// because previous_snapshot points at the collection itself, which needs an id
// that only exists after the first save.
func saveSnapshotsCollection(app core.App, cycles *core.Collection) (*core.Collection, error) {
	snapshots := core.NewBaseCollection("snapshots")
	snapshots.Fields.Add(
		&core.RelationField{Name: fieldCycle, CollectionId: cycles.Id, Required: true, MaxSelect: 1},
		&core.TextField{Name: "schema_version", Required: true},
		&core.JSONField{Name: fieldPayload, Required: true},
		&core.TextField{Name: "sha256", Required: true},
	)
	snapshots.Indexes = append(snapshots.Indexes, "CREATE UNIQUE INDEX `idx_snapshots_cycle` ON `snapshots` (`cycle`)")
	if err := app.Save(snapshots); err != nil {
		return nil, err
	}
	snapshots.Fields.Add(&core.RelationField{Name: "previous_snapshot", CollectionId: snapshots.Id, MaxSelect: 1})
	if err := app.Save(snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func newEventsCollection(snapshots *core.Collection) *core.Collection {
	events := core.NewBaseCollection("events")
	events.Fields.Add(
		&core.RelationField{Name: "snapshot", CollectionId: snapshots.Id, Required: true, MaxSelect: 1},
		&core.TextField{Name: "type", Required: true},
		&core.TextField{Name: "entity_type", Required: true},
		&core.TextField{Name: "entity_key", Required: true},
		&core.TextField{Name: "transition", Required: true},
		&core.TextField{Name: "occurrence_key", Required: true},
		&core.TextField{Name: "idempotency_key", Required: true},
		&core.JSONField{Name: fieldPayload, Required: true},
		&core.DateField{Name: "occurred_at", Required: true},
	)
	events.Indexes = append(events.Indexes, "CREATE UNIQUE INDEX `idx_events_idempotency_key` ON `events` (`idempotency_key`)")
	return events
}

func newMatchesCollection(events *core.Collection) *core.Collection {
	matches := core.NewBaseCollection("event_rule_matches")
	matches.Fields.Add(
		&core.RelationField{Name: fieldEvent, CollectionId: events.Id, Required: true, MaxSelect: 1},
		&core.TextField{Name: fieldRuleCode, Required: true},
		&core.SelectField{Name: fieldStatus, Values: []string{"pending"}, Required: true},
	)
	matches.Indexes = append(matches.Indexes, "CREATE UNIQUE INDEX `idx_event_rule_matches_event_rule` ON `event_rule_matches` (`event`, `rule_code`)")
	return matches
}
