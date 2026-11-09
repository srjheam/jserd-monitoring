package monitoring

import (
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func BuildMeta(app core.App) (*Meta, error) {
	// meta starts as the overview before any cycle is read: unavailable, with
	// no conference and no count.
	meta := &Meta{GeneratedAt: time.Now().UTC(), Availability: StateUnavailable, Conferences: []Conference{}, Aggregates: Aggregates{Metrics: map[string]int{}, Triage: emptyTriage(), Statuses: []string{}, Editors: []EditorFrequency{}}}
	attempt, err := latestCycle(app, "")
	if err != nil {
		return nil, err
	}
	if attempt != nil {
		meta.LatestAttempt = cycleView(attempt)
	}
	if err := addConferences(app, meta); err != nil {
		return nil, err
	}
	return addLatestSuccess(app, meta, attempt)
}

func addConferences(app core.App, meta *Meta) error {
	conferences, err := loadConferences(app)
	if err != nil {
		return err
	}
	meta.Conferences = conferences
	return nil
}

// addLatestSuccess fills the availability and the counts that the latest
// succeeded cycle implies.
func addLatestSuccess(app core.App, meta *Meta, attempt *core.Record) (*Meta, error) {
	success, snapshot, err := latestSuccess(app)
	if err != nil {
		return nil, err
	}
	if success == nil || snapshot == nil {
		meta.Availability = availabilityWithoutSuccess(attempt)
		return meta, nil
	}
	meta.LatestSuccess = cycleView(success)
	return addAggregates(app, meta, success)
}

// availabilityWithoutSuccess is the state to report when no cycle succeeded
// yet. A failed latest attempt means unavailable. Anything else means empty.
func availabilityWithoutSuccess(attempt *core.Record) State {
	if attempt != nil && attempt.GetString("status") == "failed" {
		return StateUnavailable
	}
	return StateEmpty
}

// addAggregates fills the side-screen counts. A projection that holds no
// submission at all is empty, whatever the cycle reported.
func addAggregates(app core.App, meta *Meta, success *core.Record) (*Meta, error) {
	total, aggregates, err := loadAggregates(app)
	if err != nil {
		return nil, err
	}
	meta.Availability = freshness(meta.GeneratedAt, success)
	if total == 0 {
		meta.Availability = StateEmpty
		return meta, nil
	}
	meta.Aggregates = aggregates
	return meta, nil
}

// freshness reports how current the projection is. A cycle that finished more
// than two days ago leaves the projection stale.
func freshness(generatedAt time.Time, success *core.Record) State {
	if generatedAt.Sub(success.GetDateTime("finished_at").Time()) > 48*time.Hour {
		return StateStale
	}
	return StateAvailable
}
