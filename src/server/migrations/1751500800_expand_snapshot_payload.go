package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

const snapshotPayloadMaxSize int64 = 64 << 20

func init() {
	migrations.Register(func(app core.App) error {
		return setSnapshotPayloadMaxSize(app, snapshotPayloadMaxSize)
	}, func(app core.App) error {
		return setSnapshotPayloadMaxSize(app, 0)
	})
}

func setSnapshotPayloadMaxSize(app core.App, maxSize int64) error {
	snapshots, err := app.FindCollectionByNameOrId("snapshots")
	if err != nil {
		return err
	}
	payload, ok := snapshots.Fields.GetByName(fieldPayload).(*core.JSONField)
	if !ok {
		return fmt.Errorf("snapshots payload field is invalid")
	}
	payload.MaxSize = maxSize
	return app.Save(snapshots)
}
