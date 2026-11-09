package monitoring

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

// sentBackToWorkBody is the fixed journal entry "send back to work" writes.
// The action asks the chief editor for no reason, so the entry is one fixed
// sentence rather than free text.
const sentBackToWorkBody = "Sent back to work."

// handleSendBackToWork clears a tracking record's up to date fields and
// writes the fixed journal entry that explains it, in one transaction.
func handleSendBackToWork(event *core.RequestEvent) error {
	ojsSubmissionID := event.Request.PathValue("id")
	entry, err := setUpToDateTx(event, ojsSubmissionID, upToDateMark{}, sentBackToWorkBody)
	if err != nil {
		return event.BadRequestError("Unable to send submission back to work", nil)
	}
	return event.JSON(http.StatusOK, entry)
}
