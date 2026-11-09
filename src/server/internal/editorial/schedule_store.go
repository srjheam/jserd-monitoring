package editorial

import (
	"server/internal/collections"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// ScheduleStore persists the forward-only next_summary_due_at timestamp in
// the one-row editorial_schedule table.
type ScheduleStore struct{ App core.App }

// NextSummaryDueAt returns the stored timestamp and whether one is set. It
// returns ok=false when no row exists yet or the timestamp was never set.
func (s ScheduleStore) NextSummaryDueAt() (time.Time, bool, error) {
	record, err := s.row()
	if err != nil || record == nil {
		return time.Time{}, false, err
	}
	due := record.GetDateTime("next_summary_due_at")
	if due.IsZero() {
		return time.Time{}, false, nil
	}
	return due.Time(), true, nil
}

// SetNextSummaryDueAt persists due as the schedule's one row, creating it on
// the first call.
func (s ScheduleStore) SetNextSummaryDueAt(due time.Time) error {
	record, err := s.row()
	if err != nil {
		return err
	}
	if record == nil {
		collection, err := s.App.FindCollectionByNameOrId(collections.EditorialSchedule)
		if err != nil {
			return err
		}
		record = core.NewRecord(collection)
	}
	record.Set("next_summary_due_at", due)
	return s.App.Save(record)
}

func (s ScheduleStore) row() (*core.Record, error) {
	records, err := s.App.FindRecordsByFilter(collections.EditorialSchedule, "", "", 1, 0)
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return records[0], nil
}
