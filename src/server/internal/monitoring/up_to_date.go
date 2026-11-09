package monitoring

import (
	"errors"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// setAsidePrefix opens the one journal entry markUpToDate writes. The client
// (src/client/src/lib/side-screens.ts) finds it again by these exact words,
// so the two must stay identical.
const setAsidePrefix = "Set aside"

type markUpToDateBody struct {
	Today  string `json:"today"`
	Until  string `json:"until"`
	Reason string `json:"reason"`
}

// handleMarkUpToDate stamps a tracking record with the current finding
// digest and writes the journal entry that explains it, in one transaction.
func handleMarkUpToDate(reqEvent *core.RequestEvent) error {
	body, err := parseMarkUpToDateBody(reqEvent)
	if err != nil {
		return reqEvent.BadRequestError(err.Error(), nil)
	}
	digest, err := requireFindingDigest(reqEvent)
	if err != nil {
		return err
	}
	mark := upToDateMark{at: body.Today, digest: digest, until: body.Until}
	entry, err := setUpToDateTx(reqEvent, reqEvent.Request.PathValue("id"), mark, setAsideBody(body.Until, body.Reason))
	if err != nil {
		return reqEvent.BadRequestError("Unable to mark submission up to date", nil)
	}
	return reqEvent.JSON(http.StatusOK, entry)
}

func parseMarkUpToDateBody(e *core.RequestEvent) (markUpToDateBody, error) {
	var body markUpToDateBody
	if err := e.BindBody(&body); err != nil {
		return body, errors.New("a today date is required")
	}
	if body.Today == "" {
		return body, errors.New("a today date is required")
	}
	return body, nil
}

// requireFindingDigest loads the submission's current finding digest, or an
// already-formed API error when the submission cannot be loaded.
func requireFindingDigest(reqEvent *core.RequestEvent) (string, error) {
	submission, found, err := FindSubmission(reqEvent.App, reqEvent.Request.PathValue("id"))
	if err != nil {
		return "", reqEvent.InternalServerError("Unable to load submission", nil)
	}
	if !found {
		return "", reqEvent.NotFoundError("Submission not found", nil)
	}
	return submission.FindingDigest, nil
}

// upToDateMark is the whole of what a chief editor's set-aside decision puts on
// a tracking record: the day the decision was made, the findings it was made
// against, and the day the submission returns. Sending a submission back to
// work is the same decision withdrawn, so it writes the zero value.
type upToDateMark struct {
	at     string
	digest string
	until  string
}

// setUpToDateTx writes one set-aside decision and the journal entry that
// explains it, in one transaction. Marking a submission up to date and sending
// it back to work both go through here, so a fourth up_to_date_* column cannot
// be set by one and left stale by the other.
func setUpToDateTx(event *core.RequestEvent, ojsSubmissionID string, mark upToDateMark, entryBody string) (*core.Record, error) {
	var entry *core.Record
	err := event.App.RunInTransaction(func(txApp core.App) error {
		tracking, err := findOrCreateTrackingRecord(txApp, ojsSubmissionID)
		if err != nil {
			return err
		}
		tracking.Set("up_to_date_at", mark.at)
		tracking.Set("up_to_date_digest", mark.digest)
		tracking.Set("up_to_date_until", mark.until)
		if err := txApp.Save(tracking); err != nil {
			return err
		}
		entry, err = appendJournalEntryRecord(txApp, ojsSubmissionID, entryBody, event.Auth)
		return err
	})
	return entry, err
}

// setAsideBody composes the exact sentence the Up to date screen finds again
// by its opening words (side-screens.ts's settingAsideEntry).
func setAsideBody(until, reason string) string {
	sentence := setAsidePrefix + "."
	if until != "" {
		sentence = setAsidePrefix + " until " + until + "."
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return sentence
	}
	return sentence + " " + reason
}
