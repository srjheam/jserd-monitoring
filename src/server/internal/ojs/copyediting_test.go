package ojs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

var copyeditorGridCases = []struct {
	name string
	body string
	want bool
}{
	{
		name: "copyeditor participant",
		body: participantGrid(`<tr id="7-row-7" class="gridRow category default_category_style"><td><span class="label">Copyeditor</span></td></tr>`, `<tr id="7-row-21282" class="gridRow"><td><span class="label">Person</span></td></tr>`),
		want: true,
	},
	{
		name: "other category participant",
		body: participantGrid(`<tr id="3-row-3" class="gridRow category default_category_style"><td><span class="label">Journal editor</span></td></tr>`, `<tr id="3-row-99" class="gridRow has_extras"><td><span class="label">Person</span></td></tr>`),
	},
	{
		name: "copyeditor between other categories",
		body: multiCategoryParticipantGrid(),
		want: true,
	},
	{
		name: "Portuguese copyeditor participant",
		body: participantGrid(`<tr id="7-row-7" class="gridRow category default_category_style"><td><span class="label"> Editor de texto </span></td></tr>`, `<tr id="7-row-21282" class="gridRow has_extras"><td><span class="label">Person</span></td></tr>`),
		want: true,
	},
	{
		name: "assignment id matches category id",
		body: participantGrid(`<tr id="7-row-7" class="gridRow category default_category_style"><td><span class="label">Copyeditor</span></td></tr>`, `<tr id="7-row-7" class="gridRow"><td><span class="label">Person</span></td></tr>`),
		want: true,
	},
	{
		name: "explicit empty grid means no participants",
		body: emptyParticipantGrid(),
	},
}

func TestParseCopyeditorGridRequiresParticipantUnderCopyeditorCategory(t *testing.T) {
	for _, testCase := range copyeditorGridCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := parseCopyeditorGrid(testCase.body)
			if err != nil || got != testCase.want {
				t.Fatalf("assigned=%v err=%v, want assigned=%v", got, err, testCase.want)
			}
		})
	}
}

func TestParseCopyeditorGridRejectsUnknownMarkup(t *testing.T) {
	for _, body := range []string{
		"<html><body>Login</body></html>",
		`<div id="not-the-participant-grid" class="pkp_controllers_grid"><table id="not-the-participant-grid-table"></table></div>`,
		`<div id="component-grid-users-stageparticipant-stageparticipantgrid-123" class="pkp_controllers_grid"><table id="other-table"></table></div>`,
		participantGrid("", ""),
		participantGrid(`<tr id="7-row-7" class="gridRow category"><td><span class="label">Copyeditor</span></td></tr>`, ""),
	} {
		if _, err := parseCopyeditorGrid(body); err == nil {
			t.Fatalf("markup %q was accepted", body)
		}
	}
}

func TestSubmissionDetailFetchesCopyeditorGridOnlyForActiveCopyediting(t *testing.T) {
	fixture := &copyeditingDetailFixture{}
	server := httptest.NewServer(fixture)
	defer server.Close()
	client := newTestClient(t, server.URL+"/index.php")
	for _, submissionID := range []string{"600348", "600453", "600454", "600455"} {
		detail, err := client.SubmissionDetail(context.Background(), submissionID)
		if err != nil {
			t.Fatalf("detail %s: %v", submissionID, err)
		}
		if submissionID == "600348" {
			if detail.CopyeditorAssigned == nil || !*detail.CopyeditorAssigned {
				t.Fatalf("copyeditor=%v, want true", detail.CopyeditorAssigned)
			}
		} else if detail.CopyeditorAssigned != nil {
			t.Fatalf("detail %s copyeditor=%v, want nil", submissionID, detail.CopyeditorAssigned)
		}
	}
	if fixture.gridRequests != 1 {
		t.Fatalf("grid requests=%d, want 1", fixture.gridRequests)
	}
}

type copyeditingDetailFixture struct {
	gridRequests int
}

func (f *copyeditingDetailFixture) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if f.writeDetail(resp, req) || f.writeGrid(resp, req) {
		return
	}
	http.NotFound(resp, req)
}

func (f *copyeditingDetailFixture) writeDetail(resp http.ResponseWriter, req *http.Request) bool {
	statuses := map[string]string{
		"600348": `{"stageId":4,"status":1}`,
		"600453": `{"stageId":4,"status":5}`,
		"600454": `{"stageId":5,"status":1}`,
		"600455": `{"stageId":3,"status":1}`,
	}
	body, ok := statuses[strings.TrimPrefix(req.URL.Path, "/index.php/jserd/api/v1/submissions/")]
	if !ok {
		return false
	}
	writeJSON(resp, body)
	return true
}

func (f *copyeditingDetailFixture) writeGrid(resp http.ResponseWriter, req *http.Request) bool {
	if req.URL.Path != "/index.php/jserd/$$$call$$$/grid/users/stage-participant/stage-participant-grid/fetch-grid" {
		return false
	}
	f.gridRequests++
	if req.Method != http.MethodGet || req.URL.Query().Get("submissionId") != "600348" || req.URL.Query().Get("stageId") != strconv.Itoa(copyeditingWorkflowStageID) {
		return false
	}
	content := participantGrid(`<tr id="7-row-7" class="gridRow category"><td><span class="label">Copyeditor</span></td></tr>`, `<tr id="7-row-21282" class="gridRow has_extras"><td><span class="label">Person</span></td></tr>`)
	body := `{"status":true,"content":` + jsonString(content) + `}`
	writeJSON(resp, body)
	return true
}

func TestCopyeditorGridFailsClosed(t *testing.T) {
	for _, response := range []string{
		`{"status":false,"content":""}`,
		`{"status":true,"content":"<html><body>Login</body></html>"}`,
		`{"status":true,"content":"not html"}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if strings.Contains(req.URL.Path, "/api/v1/submissions/") {
				resp.Header().Set("Content-Type", "application/json")
				_, _ = resp.Write([]byte(`{"stageId":4,"status":1}`))
				return
			}
			resp.Header().Set("Content-Type", "application/json")
			_, _ = resp.Write([]byte(response))
		}))
		client := newTestClient(t, server.URL+"/index.php")
		detail, err := client.SubmissionDetail(context.Background(), "600348")
		server.Close()
		if err == nil || detail.CopyeditorAssigned != nil {
			t.Fatalf("response %q detail=%+v err=%v; want error and unknown", response, detail, err)
		}
	}
}

func TestCopyeditorGridURLRejectsInvalidSubmissionID(t *testing.T) {
	client := &Client{journalContext: "jserd"}
	for _, id := range []string{"", "0", "-1", "../1"} {
		if _, err := client.copyeditorGridURL(id); err == nil {
			t.Fatalf("submission id %q accepted", id)
		}
	}
}

func participantGrid(categories, participants string) string {
	const gridID = "component-grid-users-stageparticipant-stageparticipantgrid-123"
	const rowPrefix = "component-grid-users-stageparticipant-stageparticipantgrid-category-"
	categories = strings.ReplaceAll(categories, `id="`, `id="`+rowPrefix)
	participants = strings.ReplaceAll(participants, `id="`, `id="`+rowPrefix)
	return `<div id="` + gridID + `" class="pkp_controllers_grid pkp_grid_category"><table id="` + gridID + `-table"><tbody>` + categories + `</tbody></table><div class="scrollable"><table><tbody>` + participants + `</tbody></table></div></div>`
}

func emptyParticipantGrid() string {
	const gridID = "component-grid-users-stageparticipant-stageparticipantgrid-123"
	return `<div id="` + gridID + `" class="pkp_controllers_grid pkp_grid_category"><table id="` + gridID + `-table"><tbody class="empty"><tr><td>No Items</td></tr></tbody></table></div>`
}

func multiCategoryParticipantGrid() string {
	const gridID = "component-grid-users-stageparticipant-stageparticipantgrid-123"
	const rowPrefix = "component-grid-users-stageparticipant-stageparticipantgrid-category-"
	category := func(id, label string) string {
		return `<tr id="` + rowPrefix + id + `-row-` + id + `" class="gridRow category"><td><span class="label">` + label + `</span></td></tr>`
	}
	participant := func(categoryID, assignmentID string) string {
		return `<tr id="` + rowPrefix + categoryID + `-row-` + assignmentID + `" class="gridRow"><td><span class="label">Person</span></td></tr>`
	}
	categoryTable := func(id, label string) string {
		return `<table><tbody>` + category(id, label) + `</tbody></table>`
	}
	participantTable := func(categoryID, assignmentID string) string {
		return `<div class="scrollable"><table><tbody>` + participant(categoryID, assignmentID) + `</tbody></table></div>`
	}
	body := `<table id="` + gridID + `-table"><tbody>` + category("3", "Journal editor") + `</tbody></table>` + participantTable("3", "18923") +
		categoryTable("7", "Copyeditor") + participantTable("7", "21282") + categoryTable("14", "Author") + participantTable("14", "18124")
	return `<div id="` + gridID + `" class="pkp_controllers_grid pkp_grid_category">` + body + `</div>`
}

func writeJSON(resp http.ResponseWriter, body string) {
	resp.Header().Set("Content-Type", "application/json")
	_, _ = resp.Write([]byte(body))
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
