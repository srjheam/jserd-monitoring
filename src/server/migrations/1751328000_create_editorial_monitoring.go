package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(createEditorialMonitoring, dropEditorialMonitoring)
}

// createEditorialMonitoring creates the collections the editorial summary
// e-mail and the manual collection request work on.
func createEditorialMonitoring(app core.App) error {
	cycles, err := app.FindCollectionByNameOrId("collection_cycles")
	if err != nil {
		return err
	}
	runs := newEditorialRunsCollection(cycles)
	if err := app.Save(runs); err != nil {
		return err
	}
	items := newEditorialItemsCollection(runs)
	chiefs := newChiefEditorsCollection()
	attempts := newEditorialAttemptsCollection(runs)
	requests := newManualRequestsCollection(cycles, runs)
	return saveCollections(app, items, chiefs, attempts, requests)
}

func dropEditorialMonitoring(app core.App) error {
	return dropCollections(app, "manual_collection_requests", "editorial_delivery_attempts", "chief_editors", collectionEditorialItems, collectionEditorialRuns)
}

func newEditorialRunsCollection(cycles *core.Collection) *core.Collection {
	runs := core.NewBaseCollection(collectionEditorialRuns)
	runs.Fields.Add(
		&core.TextField{Name: "window_key", Required: true}, &core.DateField{Name: "period_start", Required: true}, &core.DateField{Name: "period_end", Required: true},
		&core.SelectField{Name: "source", Values: []string{"scheduled", "recovery", "manual"}, Required: true}, &core.SelectField{Name: fieldStatus, Values: []string{"completed", statusValueFailed}, Required: true},
		&core.RelationField{Name: fieldCycle, CollectionId: cycles.Id, MaxSelect: 1}, &core.NumberField{Name: "item_count"}, &core.TextField{Name: fieldSanitizedError},
	)
	runs.Indexes = append(runs.Indexes, "CREATE UNIQUE INDEX `idx_editorial_runs_window_key` ON `editorial_runs` (`window_key`)")
	return runs
}

func newEditorialItemsCollection(runs *core.Collection) *core.Collection {
	items := core.NewBaseCollection(collectionEditorialItems)
	items.Fields.Add(&core.RelationField{Name: fieldRun, CollectionId: runs.Id, Required: true, MaxSelect: 1}, &core.TextField{Name: "submission_key", Required: true}, &core.TextField{Name: "title"}, &core.TextField{Name: "url"}, &core.TextField{Name: "date_submitted"}, &core.JSONField{Name: "findings", Required: true})
	items.Indexes = append(items.Indexes, "CREATE UNIQUE INDEX `idx_editorial_items_run_submission` ON `editorial_items` (`run`, `submission_key`)")
	return items
}

func newChiefEditorsCollection() *core.Collection {
	chiefs := core.NewBaseCollection("chief_editors")
	chiefs.Fields.Add(&core.TextField{Name: fieldName, Required: true}, &core.EmailField{Name: "email", Required: true}, &core.BoolField{Name: fieldActive}, &core.TextField{Name: "ojs_reference"})
	return chiefs
}

func newEditorialAttemptsCollection(runs *core.Collection) *core.Collection {
	attempts := core.NewBaseCollection("editorial_delivery_attempts")
	attempts.Fields.Add(&core.RelationField{Name: fieldRun, CollectionId: runs.Id, Required: true, MaxSelect: 1}, &core.JSONField{Name: "recipients", Required: true}, &core.TextField{Name: "subject"}, &core.TextField{Name: "template_version"}, &core.TextField{Name: "template_hash"}, &core.SelectField{Name: fieldStatus, Values: []string{statusValueSent, statusValueBlocked, statusValueFailed}, Required: true}, &core.TextField{Name: fieldSanitizedError})
	attempts.Indexes = append(attempts.Indexes, "CREATE UNIQUE INDEX `idx_editorial_delivery_attempts_run` ON `editorial_delivery_attempts` (`run`)")
	return attempts
}

func newManualRequestsCollection(cycles, runs *core.Collection) *core.Collection {
	requests := core.NewBaseCollection("manual_collection_requests")
	requests.Fields.Add(&core.SelectField{Name: fieldStatus, Values: []string{"pending", "running", "succeeded", statusValueFailed}, Required: true}, &core.BoolField{Name: "confirmed", Required: true}, &core.TextField{Name: fieldOperator, Required: true}, &core.TextField{Name: "note"}, &core.RelationField{Name: fieldCycle, CollectionId: cycles.Id, MaxSelect: 1}, &core.RelationField{Name: fieldRun, CollectionId: runs.Id, MaxSelect: 1}, &core.TextField{Name: fieldSanitizedError})
	return requests
}
