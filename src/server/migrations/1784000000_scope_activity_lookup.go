package migrations

import (
	"slices"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

// activityLookupEventIndexes and activityLookupAttemptIndexes let the
// submission activity route (FindActivity in internal/monitoring/activity.go)
// find the events and delivery attempts of one submission with an index seek
// instead of a full table scan. Without them, every request scanned all of
// email_delivery_attempts.
var activityLookupEventIndexes = []string{
	"CREATE INDEX `idx_events_entity_key` ON `events` (`entity_key`)",
	// Matches the exact expression PocketBase's JSON field filter resolver
	// generates for `payload.submissionId`, so a query built from that filter
	// (or the equivalent raw SQL) can use this index.
	"CREATE INDEX `idx_events_payload_submission_id` ON `events` ((CASE WHEN json_valid(`payload`) THEN JSON_EXTRACT(`payload`, '$.submissionId') ELSE JSON_EXTRACT(json_object('pb', `payload`), '$.pbsubmissionId') END))",
}

var activityLookupAttemptIndexes = []string{
	"CREATE INDEX `idx_email_delivery_attempts_event` ON `email_delivery_attempts` (`event`)",
}

func init() {
	migrations.Register(scopeActivityLookup, unscopeActivityLookup)
}

func scopeActivityLookup(app core.App) error {
	events, err := app.FindCollectionByNameOrId("events")
	if err != nil {
		return err
	}
	events.Indexes = append(events.Indexes, activityLookupEventIndexes...)
	if err := app.Save(events); err != nil {
		return err
	}

	attempts, err := app.FindCollectionByNameOrId("email_delivery_attempts")
	if err != nil {
		return err
	}
	attempts.Indexes = append(attempts.Indexes, activityLookupAttemptIndexes...)
	return app.Save(attempts)
}

func unscopeActivityLookup(app core.App) error {
	if err := dropIndexes(app, "events", activityLookupEventIndexes); err != nil {
		return err
	}
	return dropIndexes(app, "email_delivery_attempts", activityLookupAttemptIndexes)
}

// dropIndexes removes the named index definitions from one collection.
func dropIndexes(app core.App, collectionName string, drop []string) error {
	collection, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		return err
	}
	kept := make([]string, 0, len(collection.Indexes))
	for _, index := range collection.Indexes {
		if !slices.Contains(drop, index) {
			kept = append(kept, index)
		}
	}
	collection.Indexes = kept
	return app.Save(collection)
}
