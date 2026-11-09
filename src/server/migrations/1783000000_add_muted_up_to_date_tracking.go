package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addMutedUpToDateTracking, dropMutedUpToDateTracking)
}

// addMutedUpToDateTracking adds the fields that let a chief editor mute a
// submission (an editorial event owns it and the assigned editor is
// trusted) or set it aside as up to date (nothing needs a chief editor for
// now, optionally until a return date).
func addMutedUpToDateTracking(app core.App) error {
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		return err
	}
	tracking.Fields.Add(
		&core.BoolField{Name: fieldMuted},
		&core.TextField{Name: fieldMutedDigest},
		&core.DateField{Name: "up_to_date_at"},
		&core.TextField{Name: fieldUpToDateDigest},
		&core.DateField{Name: "up_to_date_until"},
	)
	return app.Save(tracking)
}

func dropMutedUpToDateTracking(app core.App) error {
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		return err
	}
	for _, field := range []string{fieldMuted, fieldMutedDigest, "up_to_date_at", fieldUpToDateDigest, "up_to_date_until"} {
		tracking.Fields.RemoveByName(field)
	}
	return app.Save(tracking)
}
