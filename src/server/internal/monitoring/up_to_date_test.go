package monitoring

import (
	"fmt"
	"net/http"
	"server/internal/collections"
	"server/internal/normalizer"
	"strings"
	"testing"
	"time"
)

// TestMarkUpToDateWritesTrackingAndJournalAtomically checks that the atomic
// route stamps the tracking record and writes the one journal entry the Up
// to date screen reads to explain it, in the same request.
func TestMarkUpToDateWritesTrackingAndJournalAtomically(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	// finding_digest is stamped by the materialization pipeline, not by a
	// hand-built row, so the fixture goes through a real snapshot.
	saveSnapshot(t, app, normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Title: testSubmissionTitle, Status: statusQueued}}}, time.Now().UTC())

	body := []byte(`{"today":"2026-09-09","until":"2026-10-09","reason":"waiting on the author"}`)
	response := apiCall(mux, http.MethodPost, "/api/jserd/monitoring/submissions/42/up-to-date", body, token)
	if response.Code != http.StatusOK {
		t.Fatalf("mark up to date = %d %s", response.Code, response.Body.String())
	}

	tracking, err := app.FindFirstRecordByData(collections.SubmissionTracking, "ojs_submission_id", "42")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tracking.GetString("up_to_date_at"), "2026-09-09") ||
		!strings.HasPrefix(tracking.GetString("up_to_date_until"), "2026-10-09") {
		t.Fatalf("tracking=%#v", tracking.PublicExport())
	}
	if tracking.GetString("up_to_date_digest") == "" {
		t.Fatal("up_to_date_digest must be stamped")
	}

	journal, err := app.FindRecordsByFilter(collections.SubmissionJournal, "ojs_submission_id = '42'", "", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(journal) != 1 || journal[0].GetString("body") != "Set aside until 2026-10-09. waiting on the author" {
		t.Fatalf("journal=%#v", journal)
	}
}

// TestSendBackToWorkClearsTrackingAndWritesJournal checks that the atomic
// route clears the three up to date fields and writes a fixed journal entry,
// reversing a previous "mark up to date".
func TestSendBackToWorkClearsTrackingAndWritesJournal(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	saveSnapshot(t, app, normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Title: testSubmissionTitle, Status: statusQueued}}}, time.Now().UTC())
	markBody := []byte(`{"today":"2026-09-09","until":"","reason":""}`)
	if response := apiCall(mux, http.MethodPost, "/api/jserd/monitoring/submissions/42/up-to-date", markBody, token); response.Code != http.StatusOK {
		t.Fatalf("mark up to date = %d %s", response.Code, response.Body.String())
	}

	response := apiCall(mux, http.MethodPost, "/api/jserd/monitoring/submissions/42/send-back-to-work", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("send back to work = %d %s", response.Code, response.Body.String())
	}

	tracking, err := app.FindFirstRecordByData(collections.SubmissionTracking, "ojs_submission_id", "42")
	if err != nil {
		t.Fatal(err)
	}
	if tracking.GetString("up_to_date_at") != "" || tracking.GetString("up_to_date_digest") != "" || tracking.GetString("up_to_date_until") != "" {
		t.Fatalf("tracking not cleared=%#v", tracking.PublicExport())
	}

	journal, err := app.FindRecordsByFilter(collections.SubmissionJournal, "ojs_submission_id = '42'", "-entry_date", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(journal) != 1 || journal[0].GetString("body") != sentBackToWorkBody {
		t.Fatalf("journal=%#v", journal)
	}
}

// TestUpToDateFieldsRejectedFromDirectClientWrites checks that the three up
// to date fields cannot be set through PocketBase's own record CRUD API,
// which would set a tracking field without the paired journal entry: only
// the two atomic routes above may write them.
func TestUpToDateFieldsRejectedFromDirectClientWrites(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	seedRowFull(t, app, "42", testSubmissionTitle, statusQueued, defaultQueue, "normal", []string{testEditorNameAlex})
	tracking := saveTracking(t, app, "42", "")

	createBody := []byte(`{"ojs_submission_id":"43","up_to_date_at":"2026-09-09"}`)
	if response := apiCall(mux, http.MethodPost, "/api/collections/submission_tracking/records", createBody, token); response.Code == http.StatusOK {
		t.Fatalf("direct create with up_to_date_at must be rejected, got %d %s", response.Code, response.Body.String())
	}

	updateBody := []byte(`{"up_to_date_digest":"forged"}`)
	path := fmt.Sprintf("/api/collections/submission_tracking/records/%s", tracking.Id)
	if response := apiCall(mux, http.MethodPatch, path, updateBody, token); response.Code == http.StatusOK {
		t.Fatalf("direct update of up_to_date_digest must be rejected, got %d %s", response.Code, response.Body.String())
	}

	// A field the two atomic routes do not own, on the same collection,
	// keeps working: the lock is scoped to the three fields, not the whole
	// record.
	muteBody := []byte(`{"muted":true}`)
	if response := apiCall(mux, http.MethodPatch, path, muteBody, token); response.Code != http.StatusOK {
		t.Fatalf("direct update of an unrelated field = %d %s", response.Code, response.Body.String())
	}
}
