package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(configureAutomatedManualRequestFields, restoreManualRequestFields)
}

// configureAutomatedManualRequestFields makes status automatic, and turns the
// operator into a relation to the signed-in Superuser.
func configureAutomatedManualRequestFields(app core.App) error {
	superusers, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		return err
	}
	operator := &core.RelationField{Name: fieldOperator, CollectionId: superusers.Id, MaxSelect: 1, Help: "Superuser autenticado; preenchido automaticamente pelo sistema."}
	return replaceManualRequestOperator(app, false, "Preenchido automaticamente pelo sistema.", operator)
}

// restoreManualRequestFields puts status and operator back the way they were
// before the automation.
func restoreManualRequestFields(app core.App) error {
	return replaceManualRequestOperator(app, true, "", &core.TextField{Name: fieldOperator, Required: true})
}

// replaceManualRequestOperator swaps the operator field for one of another
// type, and sets the status field alongside it. PocketBase cannot change a
// field's type in place, so the old field is dropped and saved first, then the
// new one is added and the stored values are written back.
func replaceManualRequestOperator(app core.App, statusRequired bool, statusHelp string, operator core.Field) error {
	operators, err := manualRequestOperators(app)
	if err != nil {
		return err
	}
	if err := dropManualRequestOperator(app, statusRequired, statusHelp); err != nil {
		return err
	}
	if err := addManualRequestOperator(app, operator); err != nil {
		return err
	}
	return restoreManualRequestOperators(app, operators)
}

// addManualRequestOperator adds the new operator field.
func addManualRequestOperator(app core.App, operator core.Field) error {
	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		return err
	}
	requests.Fields.Add(operator)
	return app.Save(requests)
}

// dropManualRequestOperator removes the operator field and applies the status
// settings.
func dropManualRequestOperator(app core.App, statusRequired bool, statusHelp string) error {
	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		return err
	}
	status, err := selectField(requests, fieldStatus)
	if err != nil {
		return err
	}
	status.Required = statusRequired
	status.Help = statusHelp
	requests.Fields.RemoveByName(fieldOperator)
	return app.Save(requests)
}

func manualRequestOperators(app core.App) (map[string]string, error) {
	records, err := app.FindAllRecords("manual_collection_requests")
	if err != nil {
		return nil, err
	}
	operators := make(map[string]string, len(records))
	for _, record := range records {
		operators[record.Id] = record.GetString(fieldOperator)
	}
	return operators, nil
}

// restoreManualRequestOperators writes the stored operator of each request
// back onto the new field.
func restoreManualRequestOperators(app core.App, operators map[string]string) error {
	for id, operator := range operators {
		if err := restoreManualRequestOperator(app, id, operator); err != nil {
			return err
		}
	}
	return nil
}

// restoreManualRequestOperator writes one stored operator back. A request that
// named no operator keeps none.
func restoreManualRequestOperator(app core.App, id, operator string) error {
	if operator == "" {
		return nil
	}
	record, err := app.FindRecordById("manual_collection_requests", id)
	if err != nil {
		return err
	}
	record.Set(fieldOperator, operator)
	return app.Save(record)
}
