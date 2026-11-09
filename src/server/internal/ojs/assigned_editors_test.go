package ojs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestParseAssignedEditorsGrid(t *testing.T) {
	body := participantGrid(
		`<tr id="5-row-5" class="gridRow category"><td><span class="label"> Associate Editor </span></td></tr>`,
		`<tr id="5-row-21" class="gridRow"><td><span class="label">Riley Park</span></td></tr><tr id="5-row-22" class="gridRow"><td><span class="label">Riley Park</span></td></tr>`,
	)
	got, err := parseAssignedEditorsGrid(body, " associate   editor ")
	if err != nil || len(got) != 1 || got[0] != "riley-park" {
		t.Fatalf("editors=%v err=%v, want [riley-park]", got, err)
	}
	got, err = parseAssignedEditorsGrid(body, "Journal editor")
	if err != nil || len(got) != 0 {
		t.Fatalf("missing group editors=%v err=%v, want empty", got, err)
	}
}

func TestParseAssignedEditorsGridRejectsMissingName(t *testing.T) {
	body := participantGrid(
		`<tr id="5-row-5" class="gridRow category"><td><span class="label">Associate Editor</span></td></tr>`,
		`<tr id="5-row-21" class="gridRow"><td></td></tr>`,
	)
	if _, err := parseAssignedEditorsGrid(body, "Associate Editor"); err == nil {
		t.Fatal("participant without a name was accepted")
	}
}

func TestAssignedEditorsRequestsRequestedStage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("submissionId") != "600440" || req.URL.Query().Get("stageId") != strconv.Itoa(3) {
			t.Errorf("query=%v, want submission 600440 at stage 3", req.URL.Query())
		}
		content := participantGrid(
			`<tr id="5-row-5" class="gridRow category"><td><span class="label">Associate Editor</span></td></tr>`,
			`<tr id="5-row-21" class="gridRow"><td><span class="label">Riley Park</span></td></tr>`,
		)
		writeJSON(resp, `{"status":true,"content":`+jsonString(content)+`}`)
	}))
	defer server.Close()
	client := newTestClient(t, server.URL+"/index.php")
	got, err := client.AssignedEditors(context.Background(), "600440", 3, "Associate Editor")
	if err != nil || len(got) != 1 || got[0] != "riley-park" {
		t.Fatalf("editors=%v err=%v, want [riley-park]", got, err)
	}
}
