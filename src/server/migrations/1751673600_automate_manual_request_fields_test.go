package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestAutomateManualRequestFieldsMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		t.Fatal(err)
	}
	status, isSelectField := requests.Fields.GetByName(fieldStatus).(*core.SelectField)
	if !isSelectField || status.Required {
		t.Fatalf("status must be optional: %#v", status)
	}
	operator, isRelationField := requests.Fields.GetByName(fieldOperator).(*core.RelationField)
	superusers, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatal(err)
	}
	if !isRelationField || operator.Required || operator.CollectionId != superusers.Id || operator.MaxSelect != 1 {
		t.Fatalf("operator must be an optional superuser relation: %#v", operator)
	}
}
