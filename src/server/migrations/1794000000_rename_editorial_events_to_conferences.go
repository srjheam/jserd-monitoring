package migrations

import (
	"fmt"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/dbutils"
)

func init() {
	migrations.Register(renameEditorialEventsToConferences, renameConferencesToEditorialEvents)
}

func renameEditorialEventsToConferences(app core.App) error {
	return renameConferenceSchema(app, "editorial_events", "conferences", "editorial_event", "conference")
}

func renameConferencesToEditorialEvents(app core.App) error {
	return renameConferenceSchema(app, "conferences", "editorial_events", "conference", "editorial_event")
}

func renameConferenceSchema(app core.App, fromCollection, toCollection, fromField, toField string) error {
	conference, err := app.FindCollectionByNameOrId(fromCollection)
	if err != nil {
		return err
	}
	conference.Name = toCollection
	for i, index := range conference.Indexes {
		parsed := dbutils.ParseIndex(index)
		parsed.IndexName = strings.ReplaceAll(parsed.IndexName, fromCollection, toCollection)
		parsed.TableName = toCollection
		conference.Indexes[i] = parsed.Build()
	}
	if err := app.Save(conference); err != nil {
		return err
	}

	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		return err
	}
	relation, ok := tracking.Fields.GetByName(fromField).(*core.RelationField)
	if !ok {
		return fmt.Errorf("submission_tracking %s field is invalid", fromField)
	}
	relation.Name = toField
	return app.Save(tracking)
}
