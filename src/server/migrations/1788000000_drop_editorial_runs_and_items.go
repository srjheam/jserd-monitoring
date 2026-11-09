package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(dropEditorialRunsAndItems, restoreEditorialRunsAndItems)
}

// dropEditorialRunsAndItems retires the run-scoped editorial tables now that
// every cycle always evaluates findings: editorial_delivery_attempts keys
// off the collection cycle that produced its findings instead of a retired
// editorial_runs row, and manual_collection_requests stops referencing a run
// record that no longer exists. One-way: the down migration restores the
// schema, not the data.
func dropEditorialRunsAndItems(app core.App) error {
	if err := dropManualRequestRun(app); err != nil {
		return err
	}
	if err := keyAttemptsByCycle(app); err != nil {
		return err
	}
	return dropCollections(app, collectionEditorialItems, collectionEditorialRuns)
}

// dropManualRequestRun removes the run reference from every manual collection
// request.
func dropManualRequestRun(app core.App) error {
	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		return err
	}
	requests.Fields.RemoveByName(fieldRun)
	return app.Save(requests)
}

// keyAttemptsByCycle keys the editorial delivery attempt off the collection
// cycle that produced its findings, in place of the retired run.
func keyAttemptsByCycle(app core.App) error {
	found, err := findCollections(app, "editorial_delivery_attempts", "collection_cycles")
	if err != nil {
		return err
	}
	attempts, cycles := found[0], found[1]
	attempts.Fields.RemoveByName(fieldRun)
	attempts.Fields.Add(&core.RelationField{Name: fieldCycle, CollectionId: cycles.Id, Required: true, MaxSelect: 1})
	attempts.Indexes = []string{"CREATE UNIQUE INDEX `idx_editorial_delivery_attempts_cycle` ON `editorial_delivery_attempts` (`cycle`)"}
	return app.Save(attempts)
}

func restoreEditorialRunsAndItems(app core.App) error {
	cycles, err := app.FindCollectionByNameOrId("collection_cycles")
	if err != nil {
		return err
	}
	runs := newEditorialRunsCollection(cycles)
	items := newEditorialItemsCollection(runs)
	if err := saveCollections(app, runs, items); err != nil {
		return err
	}
	if err := keyAttemptsByRun(app, runs); err != nil {
		return err
	}
	return restoreManualRequestRun(app, runs)
}

// keyAttemptsByRun keys the editorial delivery attempt off the restored run,
// in place of the collection cycle.
func keyAttemptsByRun(app core.App, runs *core.Collection) error {
	attempts, err := app.FindCollectionByNameOrId("editorial_delivery_attempts")
	if err != nil {
		return err
	}
	if _, ok := attempts.Fields.GetByName(fieldCycle).(*core.RelationField); !ok {
		return fmt.Errorf("editorial delivery cycle field is invalid")
	}
	attempts.Fields.RemoveByName(fieldCycle)
	attempts.Fields.Add(&core.RelationField{Name: fieldRun, CollectionId: runs.Id, Required: true, MaxSelect: 1})
	attempts.Indexes = []string{"CREATE UNIQUE INDEX `idx_editorial_delivery_attempts_run` ON `editorial_delivery_attempts` (`run`)"}
	return app.Save(attempts)
}

// restoreManualRequestRun puts the run reference back on every manual
// collection request.
func restoreManualRequestRun(app core.App, runs *core.Collection) error {
	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		return err
	}
	requests.Fields.Add(&core.RelationField{Name: fieldRun, CollectionId: runs.Id, MaxSelect: 1})
	return app.Save(requests)
}
