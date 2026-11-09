package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

const manualConfirmationHelp = "Confirma e inicia a coleta manual após criar o registro."

func init() {
	migrations.Register(func(app core.App) error {
		return setManualConfirmationHelp(app, manualConfirmationHelp)
	}, func(app core.App) error {
		return setManualConfirmationHelp(app, "")
	})
}

func setManualConfirmationHelp(app core.App, help string) error {
	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		return err
	}
	confirmed, ok := requests.Fields.GetByName("confirmed").(*core.BoolField)
	if !ok {
		return fmt.Errorf("manual collection confirmed field is invalid")
	}
	confirmed.Help = help
	return app.Save(requests)
}
