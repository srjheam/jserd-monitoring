package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addSendDigestToManualCollectionRequests, dropSendDigestFromManualCollectionRequests)
}

// addSendDigestToManualCollectionRequests lets an operator run a manual
// editorial pass that only recomputes findings, without emailing the chief
// editors' digest. The field defaults to false, so existing automation that
// does not set it keeps the "findings only" behavior; the weekly scheduled
// run always sends the digest regardless of this field.
func addSendDigestToManualCollectionRequests(app core.App) error {
	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		return err
	}
	requests.Fields.Add(&core.BoolField{Name: "send_digest"})
	return app.Save(requests)
}

func dropSendDigestFromManualCollectionRequests(app core.App) error {
	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		return err
	}
	requests.Fields.RemoveByName("send_digest")
	return app.Save(requests)
}
