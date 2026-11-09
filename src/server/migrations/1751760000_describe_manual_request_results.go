package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

var manualRequestResultHelp = map[string]string{
	fieldCycle:          "Ciclo de coleta gerado; preenchido automaticamente após a execução.",
	fieldRun:            "Execução editorial associada; preenchida automaticamente após a execução.",
	fieldSanitizedError: "Erro sanitizado da execução; permanece vazio quando a solicitação é concluída com sucesso.",
}

func init() {
	migrations.Register(func(app core.App) error {
		return setManualRequestResultHelp(app, manualRequestResultHelp)
	}, func(app core.App) error {
		return setManualRequestResultHelp(app, map[string]string{fieldCycle: "", fieldRun: "", fieldSanitizedError: ""})
	})
}

func setManualRequestResultHelp(app core.App, helpByField map[string]string) error {
	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		return err
	}
	for name, help := range helpByField {
		if err := setFieldHelp(requests, name, help); err != nil {
			return err
		}
	}
	return app.Save(requests)
}

// setFieldHelp writes the help text of one field. Only a relation field and a
// text field carry help text here.
func setFieldHelp(collection *core.Collection, name, help string) error {
	switch typed := collection.Fields.GetByName(name).(type) {
	case *core.RelationField:
		typed.Help = help
	case *core.TextField:
		typed.Help = help
	case nil:
		return fmt.Errorf("manual collection %s field is missing", name)
	default:
		return fmt.Errorf("manual collection %s field has invalid type", name)
	}
	return nil
}
