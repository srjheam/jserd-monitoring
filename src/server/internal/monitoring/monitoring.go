package monitoring

import (
	"errors"
	"net/http"
	"server/internal/collections"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func Register(app core.App) {
	RegisterHooks(app)
	registerUpToDateFieldLock(app)
	app.OnServe().BindFunc(registerRoutes)
}

// registerRoutes binds every dashboard route. Each route accepts a signed-in
// chief editor only.
func registerRoutes(serveEvent *core.ServeEvent) error {
	serveEvent.Router.GET(Route, handleMeta).Bind(apis.RequireAuth(collections.ChiefEditors))
	serveEvent.Router.GET(Route+"/submissions", handleListSubmissions).Bind(apis.RequireAuth(collections.ChiefEditors))
	serveEvent.Router.GET(Route+"/aggregates/editors", handleEditorFrequencies).Bind(apis.RequireAuth(collections.ChiefEditors))
	serveEvent.Router.GET(Route+"/submissions/{id}", handleFindSubmission).Bind(apis.RequireAuth(collections.ChiefEditors))
	serveEvent.Router.GET(Route+"/submissions/{id}/activity", handleActivity).Bind(apis.RequireAuth(collections.ChiefEditors))
	serveEvent.Router.POST(Route+"/submissions/{id}/journal", handleJournalEntry).Bind(apis.RequireAuth(collections.ChiefEditors))
	serveEvent.Router.POST(Route+"/submissions/{id}/up-to-date", handleMarkUpToDate).Bind(apis.RequireAuth(collections.ChiefEditors))
	serveEvent.Router.POST(Route+"/submissions/{id}/send-back-to-work", handleSendBackToWork).Bind(apis.RequireAuth(collections.ChiefEditors))
	return serveEvent.Next()
}

func handleMeta(e *core.RequestEvent) error {
	meta, err := BuildMeta(e.App)
	if err != nil {
		return e.InternalServerError("Unable to build monitoring meta", nil)
	}
	return e.JSON(http.StatusOK, meta)
}

func handleListSubmissions(event *core.RequestEvent) error {
	query, err := parseListQuery(event)
	if err != nil {
		return badRequestOrInternal(event, err, "Unable to list submissions")
	}
	page, err := ListSubmissions(event.App, query)
	if err != nil {
		return badRequestOrInternal(event, err, "Unable to list submissions")
	}
	return event.JSON(http.StatusOK, page)
}

func handleEditorFrequencies(event *core.RequestEvent) error {
	scope, err := parseEditorFrequencyScope(event)
	if err != nil {
		return badRequestOrInternal(event, err, "Unable to load editor frequencies")
	}
	editors, err := loadEditorFrequenciesRanked(event.App, scope)
	if err != nil {
		return event.InternalServerError("Unable to load editor frequencies", nil)
	}
	return event.JSON(http.StatusOK, editors)
}

func handleFindSubmission(event *core.RequestEvent) error {
	submission, found, err := FindSubmission(event.App, event.Request.PathValue("id"))
	if err != nil {
		return event.InternalServerError("Unable to load submission", nil)
	}
	if !found {
		return event.NotFoundError("Submission not found", nil)
	}
	return event.JSON(http.StatusOK, submission)
}

func handleActivity(e *core.RequestEvent) error {
	items, err := FindActivity(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.InternalServerError("Unable to load submission activity", nil)
	}
	return e.JSON(http.StatusOK, items)
}

func handleJournalEntry(reqEvent *core.RequestEvent) error {
	var body struct {
		Body string `json:"body"`
	}
	if err := reqEvent.BindBody(&body); err != nil || body.Body == "" {
		return reqEvent.BadRequestError("A non-empty body is required", nil)
	}
	entry, err := appendJournalEntryRecord(reqEvent.App, reqEvent.Request.PathValue("id"), body.Body, reqEvent.Auth)
	if err != nil {
		return reqEvent.BadRequestError("Unable to save journal entry", nil)
	}
	return reqEvent.JSON(http.StatusOK, entry)
}

// badRequestOrInternal answers a client mistake with its own message, and
// anything else with the operation that failed. The operation is the caller's
// to name: this function serves two routes, and an operator reading the answer
// has to be pointed at the one that failed.
func badRequestOrInternal(e *core.RequestEvent, err error, operation string) error {
	var requestErr clientError
	if errors.As(err, &requestErr) {
		return e.BadRequestError(requestErr.Error(), nil)
	}
	return e.InternalServerError(operation, nil)
}
