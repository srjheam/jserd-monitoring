package monitoring

import (
	"errors"
	"server/internal/collections"

	"github.com/pocketbase/pocketbase/core"
)

// upToDateFields are the tracking fields "mark up to date" and "send back to
// work" write together with a journal entry, atomically. A direct client
// write to any of them would set one without the other.
var upToDateFields = []string{"up_to_date_at", "up_to_date_digest", "up_to_date_until"}

var errUpToDateFieldsLocked = errors.New(
	"up_to_date_at, up_to_date_digest and up_to_date_until are written by the mark up to date and send back to work actions",
)

// registerUpToDateFieldLock keeps a chief editor's direct PocketBase SDK
// writes from setting the up to date fields outside "mark up to date" and
// "send back to work": those two routes are now the only writers that pair a
// tracking change with the journal entry the Up to date screen reads to
// explain it, and a collection update rule cannot express that pairing. Both
// routes call app.Save directly rather than going through the API request
// pipeline, so they never trigger these hooks.
func registerUpToDateFieldLock(app core.App) {
	app.OnRecordCreateRequest(collections.SubmissionTracking).BindFunc(rejectUpToDateFieldsOnCreate)
	app.OnRecordUpdateRequest(collections.SubmissionTracking).BindFunc(rejectUpToDateFieldsOnUpdate)
}

func rejectUpToDateFieldsOnCreate(event *core.RecordRequestEvent) error {
	for _, field := range upToDateFields {
		if event.Record.GetString(field) != "" {
			return errUpToDateFieldsLocked
		}
	}
	return event.Next()
}

func rejectUpToDateFieldsOnUpdate(event *core.RecordRequestEvent) error {
	original := event.Record.Original()
	for _, field := range upToDateFields {
		if event.Record.GetString(field) != original.GetString(field) {
			return errUpToDateFieldsLocked
		}
	}
	return event.Next()
}
