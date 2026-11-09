package editorial

import (
	"context"
	"encoding/json"
	"errors"
	"server/internal/collector"
	"server/internal/normalizer"
	"server/internal/ojs"
	"server/internal/snapshotpayload"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/cron"

	_ "server/migrations"
)

// fakeSchedule is a database-free stand-in for ScheduleStore.
type fakeSchedule struct {
	due  time.Time
	ok   bool
	sets []time.Time
}

func (f *fakeSchedule) Next() (time.Time, bool, error) { return f.due, f.ok, nil }
func (f *fakeSchedule) Set(t time.Time) error {
	f.sets = append(f.sets, t)
	f.due, f.ok = t, true
	return nil
}

func baseCoordinator(app *tests.TestApp) (*Coordinator, *spyDelivery, *fakeSchedule) {
	delivery := &spyDelivery{}
	sched := &fakeSchedule{}
	schedule, _ := cron.NewSchedule("0 9 * * 1")
	coordinator := &Coordinator{
		App:                 app,
		Store:               NewStore(app),
		Delivery:            delivery,
		Collect:             func(context.Context) (collector.CycleResult, error) { return collector.CycleResult{}, nil },
		Process:             func(collector.CycleResult) error { return nil },
		Schedule:            schedule,
		Now:                 func() time.Time { return time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) },
		NextSummaryDueAt:    sched.Next,
		SetNextSummaryDueAt: sched.Set,
	}
	return coordinator, delivery, sched
}

func TestRunCycleAlwaysEvaluatesFindings(t *testing.T) {
	app := editorialTestApp(t)
	saveEvidenceSnapshot(t, app, "", "42")
	coordinator, _, _ := baseCoordinator(app)
	for _, testCase := range []struct {
		source      RunSource
		forceDigest bool
	}{
		{RunSourceScheduled, false},
		{RunSourceManual, false},
		{RunSourceManual, true},
	} {
		var findingsReady bool
		coordinator.OnFindingsReady = func(core.App, string) error { findingsReady = true; return nil }
		result, err := coordinator.RunCycle(context.Background(), testCase.source, "request", testCase.forceDigest)
		if err != nil {
			t.Fatal(err)
		}
		if result.CycleID == "" || !findingsReady {
			t.Fatalf("source=%v forceDigest=%v result=%#v findingsReady=%v", testCase.source, testCase.forceDigest, result, findingsReady)
		}
	}
}

func TestRunCyclePendingFailureDoesNotBlockFindings(t *testing.T) {
	app := editorialTestApp(t)
	saveEvidenceSnapshot(t, app, "", "42")
	coordinator, _, _ := baseCoordinator(app)
	coordinator.CreatePending = func() error { return errors.New("recipient not found") }
	var findingsReady bool
	coordinator.OnFindingsReady = func(core.App, string) error { findingsReady = true; return nil }
	result, err := coordinator.RunCycle(context.Background(), RunSourceScheduled, "", false)
	if err == nil {
		t.Fatal("expected the pending-notification failure to still surface")
	}
	if result.CycleID == "" || !findingsReady {
		t.Fatalf("a pending-notification failure must not stop findings evaluation: result=%#v findingsReady=%v", result, findingsReady)
	}
}

func TestRunCycleShareMutex(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	coordinator := &Coordinator{Collect: func(context.Context) (collector.CycleResult, error) {
		close(entered)
		<-release
		return collector.CycleResult{}, errors.New("stop")
	}, Process: func(collector.CycleResult) error { return nil }}
	done := make(chan error, 1)
	go func() {
		_, err := coordinator.RunCycle(context.Background(), RunSourceScheduled, "", false)
		done <- err
	}()
	<-entered
	if _, err := coordinator.RunCycle(context.Background(), RunSourceManual, "request", true); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("RunCycle err=%v", err)
	}
	close(release)
	<-done
}

func TestForceDigestBypassesSchedule(t *testing.T) {
	app := editorialTestApp(t)
	saveEvidenceSnapshot(t, app, "", "42")
	coordinator, delivery, sched := baseCoordinator(app)
	sched.due, sched.ok = time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC), true // far in the future, not due
	if _, err := coordinator.RunCycle(context.Background(), RunSourceManual, "request", true); err != nil {
		t.Fatal(err)
	}
	if delivery.calls.Load() != 1 {
		t.Fatalf("delivery calls=%d", delivery.calls.Load())
	}
}

func TestNonForcedCycleSendsOnlyWhenDue(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		due       time.Time
		ok        bool
		wantCalls int32
	}{
		{"unset", time.Time{}, false, 0},
		{"future", time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC), true, 0},
		{"due", time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC), true, 1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			app := editorialTestApp(t)
			saveEvidenceSnapshot(t, app, "", "42")
			coordinator, delivery, sched := baseCoordinator(app)
			sched.due, sched.ok = testCase.due, testCase.ok
			if _, err := coordinator.RunCycle(context.Background(), RunSourceScheduled, "", false); err != nil {
				t.Fatal(err)
			}
			if delivery.calls.Load() != testCase.wantCalls {
				t.Fatalf("delivery calls=%d want=%d", delivery.calls.Load(), testCase.wantCalls)
			}
		})
	}
}

type failingDelivery struct{ err error }

func (f failingDelivery) Deliver(context.Context, string) error { return f.err }

type spyDelivery struct{ calls atomic.Int32 }

func (s *spyDelivery) Deliver(context.Context, string) error { s.calls.Add(1); return nil }

func runCyclePropagatesFailuresCases() []struct {
	name      string
	configure func(*Coordinator)
} {
	return []struct {
		name      string
		configure func(*Coordinator)
	}{
		{"collect", func(coordinator *Coordinator) {
			coordinator.Collect = func(context.Context) (collector.CycleResult, error) {
				return collector.CycleResult{}, errors.New("token=secret")
			}
		}},
		{"preProcessEnrich", func(coordinator *Coordinator) {
			coordinator.PreProcessEnrichers = []EnrichmentStage{func(context.Context, *normalizer.NormalizedCycle, SubmissionDetailProvider) error {
				return errors.New("token=secret")
			}}
		}},
		{"process", func(coordinator *Coordinator) {
			coordinator.Process = func(collector.CycleResult) error { return errors.New("password=secret") }
		}},
		{"pending", func(coordinator *Coordinator) {
			coordinator.CreatePending = func() error { return errors.New("cookie=secret") }
		}},
		{fieldSnapshot, func(coordinator *Coordinator) {}},
		{"delivery", func(coordinator *Coordinator) {
			coordinator.Delivery = failingDelivery{err: errors.New("authorization: bearer secret")}
			coordinator.NextSummaryDueAt = func() (time.Time, bool, error) { return time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC), true, nil }
			coordinator.SetNextSummaryDueAt = func(time.Time) error { return nil }
		}},
	}
}

func TestRunCyclePropagatesFailures(t *testing.T) {
	for _, testCase := range runCyclePropagatesFailuresCases() {
		t.Run(testCase.name, func(t *testing.T) {
			app := editorialTestApp(t)
			if testCase.name != fieldSnapshot {
				saveEvidenceSnapshot(t, app, "", "42")
			}
			schedule, _ := cron.NewSchedule("0 9 * * 1")
			coordinator := &Coordinator{App: app, Store: NewStore(app), Collect: func(context.Context) (collector.CycleResult, error) { return collector.CycleResult{}, nil }, Process: func(collector.CycleResult) error { return nil }, Schedule: schedule, Now: func() time.Time { return time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) }}
			testCase.configure(coordinator)
			_, runErr := coordinator.RunCycle(context.Background(), RunSourceManual, "failure-"+testCase.name, false)
			if runErr == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRunCycleCallsEnrichBeforeEvaluatingAndPropagatesItsFailure(t *testing.T) {
	app := editorialTestApp(t)
	saveEvidenceSnapshot(t, app, "", "42")
	coordinator, _, _ := baseCoordinator(app)
	var enrichedID string
	coordinator.Enrichers = []EnrichmentStage{func(_ context.Context, cycle *normalizer.NormalizedCycle, _ SubmissionDetailProvider) error {
		if len(cycle.Submissions) == 1 {
			enrichedID = cycle.Submissions[0].OJSSubmissionID
		}
		return nil
	}}
	if _, err := coordinator.RunCycle(context.Background(), RunSourceScheduled, "", false); err != nil {
		t.Fatal(err)
	}
	if enrichedID != "42" {
		t.Fatalf("Enrich did not see the loaded snapshot: enrichedID=%q", enrichedID)
	}

	coordinator.Enrichers = []EnrichmentStage{func(context.Context, *normalizer.NormalizedCycle, SubmissionDetailProvider) error {
		return errors.New("token=secret")
	}}
	if _, err := coordinator.RunCycle(context.Background(), RunSourceScheduled, "", false); err == nil {
		t.Fatal("expected an Enrich failure to surface")
	}
}

func TestRunCycleCallsPreProcessEnrichBeforeProcessAndPropagatesItsFailure(t *testing.T) {
	app := editorialTestApp(t)
	saveEvidenceSnapshot(t, app, "", "42")
	coordinator, _, _ := baseCoordinator(app)
	var enrichedBeforeProcess bool
	var processedID string
	coordinator.PreProcessEnrichers = []EnrichmentStage{func(_ context.Context, cycle *normalizer.NormalizedCycle, _ SubmissionDetailProvider) error {
		cycle.Reviews = []normalizer.ReviewAssignment{{SubmissionID: "42"}}
		return nil
	}}
	coordinator.Process = func(result collector.CycleResult) error {
		if len(result.Normalized.Reviews) == 1 && result.Normalized.Reviews[0].SubmissionID == "42" {
			enrichedBeforeProcess = true
			processedID = result.Normalized.Reviews[0].SubmissionID
		}
		return nil
	}
	if _, err := coordinator.RunCycle(context.Background(), RunSourceScheduled, "", false); err != nil {
		t.Fatal(err)
	}
	if !enrichedBeforeProcess || processedID != "42" {
		t.Fatalf("Process did not see PreProcessEnrich's mutation: enrichedBeforeProcess=%v processedID=%q", enrichedBeforeProcess, processedID)
	}

	coordinator.PreProcessEnrichers = []EnrichmentStage{func(context.Context, *normalizer.NormalizedCycle, SubmissionDetailProvider) error {
		return errors.New("token=secret")
	}}
	if _, err := coordinator.RunCycle(context.Background(), RunSourceScheduled, "", false); err == nil {
		t.Fatal("expected a PreProcessEnrich failure to surface")
	}
}

func TestPreProcessEnrichersRunInRegistrationOrder(t *testing.T) {
	app := editorialTestApp(t)
	saveEvidenceSnapshot(t, app, "", "42")
	coordinator, _, _ := baseCoordinator(app)
	var order []string
	coordinator.PreProcessEnrichers = []EnrichmentStage{
		func(context.Context, *normalizer.NormalizedCycle, SubmissionDetailProvider) error {
			order = append(order, "first")
			return nil
		},
		func(context.Context, *normalizer.NormalizedCycle, SubmissionDetailProvider) error {
			order = append(order, "second")
			return nil
		},
	}
	if _, err := coordinator.RunCycle(context.Background(), RunSourceScheduled, "", false); err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("order=%v, want [first second]", order)
	}
}

func TestRunCyclePassesTheSameLoaderToPreProcessEnrichersAndEnrich(t *testing.T) {
	app := editorialTestApp(t)
	saveEvidenceSnapshot(t, app, "", "42")
	coordinator, _, _ := baseCoordinator(app)
	source := &fakeSubmissionDetailSource{details: map[string]ojs.SubmissionDetail{"42": {}}}
	coordinator.OJSClient = source
	coordinator.OJSUsername, coordinator.OJSPassword = targetKindEditor, "secret"
	coordinator.PreProcessEnrichers = []EnrichmentStage{
		func(ctx context.Context, _ *normalizer.NormalizedCycle, details SubmissionDetailProvider) error {
			_, err := details.Get(ctx, "42")
			return err
		},
	}
	coordinator.Enrichers = []EnrichmentStage{func(ctx context.Context, _ *normalizer.NormalizedCycle, details SubmissionDetailProvider) error {
		_, err := details.Get(ctx, "42")
		return err
	}}
	if _, err := coordinator.RunCycle(context.Background(), RunSourceScheduled, "", false); err != nil {
		t.Fatal(err)
	}
	if len(source.fetched) != 1 {
		t.Fatalf("fetched=%v, want exactly one live fetch shared across both stages", source.fetched)
	}
	if source.loginCalls != 1 {
		t.Fatalf("loginCalls=%d, want 1", source.loginCalls)
	}
}

func TestCoordinatorUsesPersistentEvidence(t *testing.T) {
	app := editorialTestApp(t)
	current := saveEvidenceSnapshot(t, app, "", "42")
	pending := true
	payload, _ := json.Marshal(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Status: statusReview, DateSubmitted: testDate20260101, RevisionPending: &pending, Editors: []normalizer.Editor{{Email: testChiefEmail, Decisions: []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260601}}}}}}})
	payloadFile, err := snapshotpayload.NewFile(payload)
	if err != nil {
		t.Fatal(err)
	}
	current.Set(fieldPayload, payloadFile)
	if err := app.Save(current); err != nil {
		t.Fatal(err)
	}
	coordinator, _, _ := baseCoordinator(app)
	coordinator.Evaluator = Evaluator{}
	result, err := coordinator.RunCycle(context.Background(), RunSourceManual, "persistent-evidence", false)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := LatestFindingsFor(app, []string{"42"})
	if err != nil || !containsKind(findings["42"], FindingMissingRevisionIndication) {
		t.Fatalf("cycle=%s findings=%#v err=%v", result.CycleID, findings, err)
	}
}
