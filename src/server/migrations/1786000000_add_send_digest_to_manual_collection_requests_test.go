package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestAddSendDigestToManualCollectionRequestsMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		t.Fatal(err)
	}
	field, ok := requests.Fields.GetByName("send_digest").(*core.BoolField)
	if !ok {
		t.Fatal("missing send_digest bool field")
	}
	if field.Required {
		t.Fatal("send_digest must not be required")
	}

	if err := dropSendDigestFromManualCollectionRequests(app); err != nil {
		t.Fatal(err)
	}
	requests, err = app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		t.Fatal(err)
	}
	if requests.Fields.GetByName("send_digest") != nil {
		t.Fatal("send_digest survived rollback")
	}

	if err := addSendDigestToManualCollectionRequests(app); err != nil {
		t.Fatal(err)
	}
	requests, err = app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		t.Fatal(err)
	}
	if requests.Fields.GetByName("send_digest") == nil {
		t.Fatal("missing send_digest field after re-applying migration")
	}
}
