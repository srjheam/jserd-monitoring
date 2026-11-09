package migrations

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestRenameEditorialEventsMigrationPreservesRecordsAndIds(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if err := renameConferencesToEditorialEvents(app); err != nil {
		t.Fatal(err)
	}
	fixture := createLegacyEditorialEventFixture(t, app)

	if err := renameEditorialEventsToConferences(app); err != nil {
		t.Fatal(err)
	}
	assertConferenceRenameState(t, app, "conferences", "conference", fixture.collectionID, fixture.fieldID, fixture.conferenceID, fixture.trackingID)
	assertConferenceConfiguration(t, app, "conferences")
	assertLegacyConferenceNamesAbsent(t, app)
	assertConferenceNameUnique(t, app)

	if err := renameConferencesToEditorialEvents(app); err != nil {
		t.Fatal(err)
	}
	assertConferenceRenameState(t, app, "editorial_events", "editorial_event", fixture.collectionID, fixture.fieldID, fixture.conferenceID, fixture.trackingID)
	assertConferenceConfiguration(t, app, "editorial_events")
	if err := renameEditorialEventsToConferences(app); err != nil {
		t.Fatal(err)
	}
	assertConferenceRenameState(t, app, "conferences", "conference", fixture.collectionID, fixture.fieldID, fixture.conferenceID, fixture.trackingID)
	assertConferenceConfiguration(t, app, "conferences")
}

type legacyEditorialEventFixture struct {
	collectionID string
	fieldID      string
	conferenceID string
	trackingID   string
}

func createLegacyEditorialEventFixture(t *testing.T, app core.App) legacyEditorialEventFixture {
	t.Helper()
	eventCollection, err := app.FindCollectionByNameOrId("editorial_events")
	if err != nil {
		t.Fatal(err)
	}
	eventCollection.Indexes = append(eventCollection.Indexes, "CREATE INDEX `idx_editorial_events_description` ON `editorial_events` (`description`)")
	eventCollection.Indexes = append(eventCollection.Indexes, "CREATE INDEX `idx_editorial_events_literal` ON `editorial_events` (`description`) WHERE description = 'editorial_events'")
	eventCollection.ListRule = types.Pointer("@request.auth.id != ''")
	eventCollection.CreateRule = types.Pointer("@request.auth.id != ''")
	if err := app.Save(eventCollection); err != nil {
		t.Fatal(err)
	}
	trackingCollection, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	relation, ok := trackingCollection.Fields.GetByName("editorial_event").(*core.RelationField)
	if !ok {
		t.Fatal("editorial_event must be a relation field")
	}
	event := core.NewRecord(eventCollection)
	event.Set(fieldName, "SBQS")
	event.Set("description", "Conference grouping")
	if err := app.Save(event); err != nil {
		t.Fatal(err)
	}
	tracking := core.NewRecord(trackingCollection)
	tracking.Set(fieldOJSSubmissionID, "42")
	tracking.Set("editorial_event", event.Id)
	tracking.Set(fieldMuted, true)
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
	return legacyEditorialEventFixture{
		collectionID: eventCollection.Id,
		fieldID:      relation.Id,
		conferenceID: event.Id,
		trackingID:   tracking.Id,
	}
}

func assertLegacyConferenceNamesAbsent(t *testing.T, app core.App) {
	t.Helper()
	if _, err := app.FindCollectionByNameOrId("editorial_events"); err == nil {
		t.Fatal("editorial_events collection survived the rename")
	}
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	if tracking.Fields.GetByName("editorial_event") != nil {
		t.Fatal("editorial_event field survived the rename")
	}
}

func assertConferenceConfiguration(t *testing.T, app core.App, name string) {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		t.Fatal(err)
	}
	if collection.ListRule == nil || *collection.ListRule != "@request.auth.id != ''" || collection.CreateRule == nil || *collection.CreateRule != "@request.auth.id != ''" {
		t.Fatal("conference access rules changed during the rename")
	}
	indexes := strings.Join(collection.Indexes, "\n")
	for _, index := range []string{"idx_" + name + "_name", "idx_" + name + "_description", "idx_" + name + "_literal"} {
		if !strings.Contains(indexes, index) {
			t.Fatalf("conference index %s was not preserved: %v", index, collection.Indexes)
		}
	}
	if !strings.Contains(indexes, "WHERE description = 'editorial_events'") {
		t.Fatal("index predicate changed during the rename")
	}
}

func assertConferenceRenameState(t *testing.T, app core.App, collectionName, relationFieldName, collectionID, fieldID, conferenceID, trackingID string) {
	t.Helper()
	conference, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		t.Fatal(err)
	}
	if conference.Id != collectionID {
		t.Fatalf("collection ID=%q, want %q", conference.Id, collectionID)
	}
	field := conferenceField(t, app, relationFieldName)
	if field.Id != fieldID || field.CollectionId != collectionID {
		t.Fatalf("relation field=%#v, want original field and collection IDs", field)
	}
	record, err := app.FindRecordById(collectionName, conferenceID)
	if err != nil || record.GetString(fieldName) != "SBQS" || record.GetString("description") != "Conference grouping" {
		t.Fatalf("conference record=%v, err=%v", record, err)
	}
	tracking, err := app.FindRecordById("submission_tracking", trackingID)
	if err != nil || tracking.GetString(relationFieldName) != conferenceID || !tracking.GetBool(fieldMuted) {
		t.Fatalf("tracking relation/state=%v, err=%v", tracking, err)
	}
}

func conferenceField(t *testing.T, app core.App, name string) *core.RelationField {
	t.Helper()
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	field, ok := tracking.Fields.GetByName(name).(*core.RelationField)
	if !ok {
		t.Fatalf("submission_tracking.%s must be a relation field", name)
	}
	return field
}

func assertConferenceNameUnique(t *testing.T, app core.App) {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId("conferences")
	if err != nil {
		t.Fatal(err)
	}
	duplicate := core.NewRecord(collection)
	duplicate.Set(fieldName, "SBQS")
	if err := app.Save(duplicate); err == nil {
		t.Fatal("renamed conference name index must remain unique")
	}
}
