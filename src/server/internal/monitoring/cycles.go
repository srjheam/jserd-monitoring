package monitoring

import (
	"database/sql"
	"errors"
	"server/internal/audit"
	"server/internal/collections"
	"sort"

	"github.com/pocketbase/pocketbase/core"
)

func latestCycle(app core.App, status string) (*core.Record, error) {
	query := "SELECT id FROM collection_cycles"
	if status != "" {
		query += " WHERE status = {:status}"
	}
	query += " ORDER BY finished_at DESC, id DESC LIMIT 1"
	var cycleID string
	err := app.DB().NewQuery(query).Bind(map[string]any{statusFieldName: status}).Row(&cycleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return app.FindRecordById(collections.CollectionCycles, cycleID)
}

func latestSuccess(app core.App) (*core.Record, *core.Record, error) {
	var row struct {
		CycleID    string `db:"cycle_id"`
		SnapshotID string `db:"snapshot_id"`
	}
	err := app.DB().NewQuery(`SELECT cycle.id cycle_id, snapshots.id snapshot_id FROM snapshots JOIN collection_cycles cycle ON cycle.id = snapshots.cycle WHERE cycle.status = 'succeeded' ORDER BY cycle.finished_at DESC, cycle.id DESC LIMIT 1`).One(&row)
	if err != nil {
		return nil, nil, ignoreNoRows(err)
	}
	return cycleAndSnapshot(app, row.CycleID, row.SnapshotID)
}

// ignoreNoRows turns "no row matched" into no error, because an empty table is
// a normal state, not a failure.
func ignoreNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

// cycleAndSnapshot loads the cycle record and the snapshot record one row names.
func cycleAndSnapshot(app core.App, cycleID, snapshotID string) (*core.Record, *core.Record, error) {
	cycle, err := app.FindRecordById(collections.CollectionCycles, cycleID)
	if err != nil {
		return nil, nil, err
	}
	snapshot, err := app.FindRecordById(collections.Snapshots, snapshotID)
	if err != nil {
		return nil, nil, err
	}
	return cycle, snapshot, nil
}

func cycleView(record *core.Record) *Collection {
	return &Collection{ID: record.Id, Source: record.GetString("source_mode"), Status: record.GetString("status"), StartedAt: record.GetDateTime("started_at").Time(), FinishedAt: record.GetDateTime("finished_at").Time(), Failure: audit.RedactError(record.GetString("error"))}
}

func loadConferences(app core.App) ([]Conference, error) {
	records, err := app.FindAllRecords(collections.Conferences)
	if err != nil {
		return nil, err
	}
	items := make([]Conference, 0, len(records))
	for _, record := range records {
		items = append(items, Conference{ID: record.Id, Name: record.GetString("name"), Description: record.GetString("description")})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}
