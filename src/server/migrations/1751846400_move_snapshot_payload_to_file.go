package migrations

import (
	"fmt"
	"server/internal/snapshotpayload"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

const temporarySnapshotPayloadField = "payload_file"

func init() {
	migrations.Register(moveSnapshotPayloadToFile, restoreSnapshotPayloadJSON)
}

// moveSnapshotPayloadToFile moves every snapshot payload out of a JSON column
// and into a stored file. It adds the file field beside the JSON one, copies
// every payload across, then drops the JSON field and renames the file field
// over it.
func moveSnapshotPayloadToFile(app core.App) error {
	payloadFile := &core.FileField{
		Name:      temporarySnapshotPayloadField,
		MaxSize:   snapshotpayload.MaxSize,
		MaxSelect: 1,
		MimeTypes: []string{"application/json"},
		Protected: true,
		Required:  true,
		Hidden:    true,
		Help:      "Payload canônico do snapshot; gerenciado pelo sistema.",
	}
	if err := addTemporaryPayloadField(app, fieldPayload, payloadFile); err != nil {
		return err
	}
	if err := forEachSnapshot(app, copyPayloadToFile); err != nil {
		return err
	}
	return renamePayloadField(app, temporarySnapshotPayloadField, showFileField)
}

// restoreSnapshotPayloadJSON is the reverse move: out of a stored file and back
// into a JSON column.
func restoreSnapshotPayloadJSON(app core.App) error {
	if err := addTemporaryPayloadField(app, fieldPayload, &core.JSONField{Name: "payload_json", Required: true, MaxSize: snapshotpayload.MaxSize}); err != nil {
		return err
	}
	if err := forEachSnapshot(app, copyPayloadToJSON); err != nil {
		return err
	}
	return renamePayloadField(app, "payload_json", nil)
}

// addTemporaryPayloadField adds the field the payload moves into, beside the
// field it moves out of.
func addTemporaryPayloadField(app core.App, currentName string, field core.Field) error {
	snapshots, err := app.FindCollectionByNameOrId("snapshots")
	if err != nil {
		return err
	}
	if err := checkPayloadFieldType(snapshots, currentName, field); err != nil {
		return err
	}
	snapshots.Fields.Add(field)
	return app.Save(snapshots)
}

// checkPayloadFieldType refuses to move a payload that already sits in the
// target form, so running the migration twice cannot lose data.
func checkPayloadFieldType(snapshots *core.Collection, currentName string, target core.Field) error {
	current := snapshots.Fields.GetByName(currentName)
	if _, movingToFile := target.(*core.FileField); movingToFile {
		if _, ok := current.(*core.JSONField); !ok {
			return fmt.Errorf("snapshots payload field is not JSON")
		}
		return nil
	}
	if _, ok := current.(*core.FileField); !ok {
		return fmt.Errorf("snapshots payload field is not a file")
	}
	return nil
}

// forEachSnapshot applies one step to every snapshot record, in id order.
func forEachSnapshot(app core.App, step func(core.App, *core.Record) error) error {
	ids, err := snapshotRecordIDs(app)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := applyToSnapshot(app, id, step); err != nil {
			return err
		}
	}
	return nil
}

// applyToSnapshot loads one snapshot record and applies the step to it.
func applyToSnapshot(app core.App, id string, step func(core.App, *core.Record) error) error {
	record, err := app.FindRecordById("snapshots", id)
	if err != nil {
		return err
	}
	return step(app, record)
}

// copyPayloadToFile writes one snapshot's JSON payload into the stored file.
func copyPayloadToFile(app core.App, record *core.Record) error {
	file, err := snapshotpayload.NewFile([]byte(record.GetString(fieldPayload)))
	if err != nil {
		return fmt.Errorf("prepare snapshot %s payload: %w", record.Id, err)
	}
	record.Set(temporarySnapshotPayloadField, file)
	if err := app.Save(record); err != nil {
		return fmt.Errorf("migrate snapshot %s payload: %w", record.Id, err)
	}
	return nil
}

// copyPayloadToJSON writes one snapshot's stored file back into the JSON field.
func copyPayloadToJSON(app core.App, record *core.Record) error {
	payload, err := snapshotpayload.Read(app, record)
	if err != nil {
		return fmt.Errorf("read snapshot %s payload: %w", record.Id, err)
	}
	record.Set("payload_json", payload)
	if err := app.Save(record); err != nil {
		return fmt.Errorf("restore snapshot %s payload: %w", record.Id, err)
	}
	return nil
}

// showFileField makes the moved file field visible, once it takes the payload
// name.
func showFileField(field core.Field) {
	if file, ok := field.(*core.FileField); ok {
		file.Hidden = false
	}
}

// renamePayloadField drops the old payload field and renames the temporary one
// over it.
func renamePayloadField(app core.App, temporaryName string, adjust func(core.Field)) error {
	snapshots, err := app.FindCollectionByNameOrId("snapshots")
	if err != nil {
		return err
	}
	field := snapshots.Fields.GetByName(temporaryName)
	if field == nil {
		return fmt.Errorf("temporary snapshot payload field is invalid")
	}
	snapshots.Fields.RemoveByName(fieldPayload)
	field.SetName(fieldPayload)
	if adjust != nil {
		adjust(field)
	}
	return app.Save(snapshots)
}

func snapshotRecordIDs(app core.App) ([]string, error) {
	var rows []struct {
		ID string `db:"id"`
	}
	if err := app.DB().Select("id").From("snapshots").OrderBy("id").All(&rows); err != nil {
		return nil, err
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return ids, nil
}
