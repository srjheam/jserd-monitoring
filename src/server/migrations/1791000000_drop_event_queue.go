package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(dropEventQueue, restoreEventQueue)
}

// dropEventQueue removes the "event" effective_queue value: it never should
// have been its own placement, since it is set alongside daily, up_to_date,
// muted and archived, which already say everything the queue is for. Every
// row currently in "event" falls through to "daily", the same status
// queueFor now gives a submission with an assigned editorial event.
func dropEventQueue(app core.App) error {
	if _, err := app.DB().NewQuery("UPDATE ojs_submissions SET effective_queue = 'daily' WHERE effective_queue = 'event'").Execute(); err != nil {
		return err
	}
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		return err
	}
	if queue, ok := submissions.Fields.GetByName("effective_queue").(*core.SelectField); ok {
		queue.Values = []string{queueValueDaily, queueValueArchived, fieldMuted, queueValueUpToDate}
	}
	return app.Save(submissions)
}

func restoreEventQueue(app core.App) error {
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		return err
	}
	if queue, ok := submissions.Fields.GetByName("effective_queue").(*core.SelectField); ok {
		queue.Values = []string{queueValueDaily, fieldEvent, queueValueArchived, fieldMuted, queueValueUpToDate}
	}
	return app.Save(submissions)
}
