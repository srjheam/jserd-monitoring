package editorial

import (
	"encoding/json"
	"server/internal/collections"
	"server/internal/notifications"

	"github.com/pocketbase/pocketbase/core"
)

// PocketBaseDeliveryLookup answers Evaluator's DeliveryLookup interface by
// reading OJS RN4 evidence PocketBase already stores. It reads the rule matches
// and the events behind them, and asks internal/notifications about the
// delivery trail, because those collections are that package's own.
type PocketBaseDeliveryLookup struct {
	App core.App
}

// rn4MatchState is what one RN4 match says about a submission.
type rn4MatchState int

const (
	// rn4NotMine means that the match names another submission.
	rn4NotMine rn4MatchState = iota
	// rn4Sent means that every notification e-mail the match called for was
	// sent.
	rn4Sent
	// rn4Unsent means that at least one notification e-mail is still unsent.
	rn4Unsent
	// rn4Unknown means that the record trail is broken or unreadable.
	rn4Unknown
)

// rn4Decisions maps the state of one RN4 match to the answer HasUnsentRN4 must
// return at once. A state with no entry lets the scan continue.
var rn4Decisions = map[rn4MatchState][2]bool{
	rn4Unknown: {false, false},
	rn4Unsent:  {true, true},
}

// HasUnsentRN4 reports whether the submission has an RN4 match whose
// notification e-mail was never sent, and whether the submission has any RN4
// match at all. A broken record trail answers false for both.
func (l PocketBaseDeliveryLookup) HasUnsentRN4(submissionID string) (bool, bool) {
	matches, err := l.App.FindRecordsByFilter(collections.EventRuleMatches, "rule_code = 'RN4'", "", 0, 0)
	if err != nil {
		return false, false
	}
	seen := map[rn4MatchState]bool{}
	for _, match := range matches {
		state := l.rn4State(match, submissionID)
		if answer, decided := rn4Decisions[state]; decided {
			return answer[0], answer[1]
		}
		seen[state] = true
	}
	return false, seen[rn4Sent]
}

// rn4State reports what one RN4 match says about a submission.
func (l PocketBaseDeliveryLookup) rn4State(match *core.Record, submissionID string) rn4MatchState {
	mine, ok := l.matchNamesSubmission(match, submissionID)
	if !ok {
		return rn4Unknown
	}
	if !mine {
		return rn4NotMine
	}
	return rn4States[notifications.MatchEmailState(l.App, match.Id)]
}

// rn4States translates the delivery answer into this file's vocabulary, which
// carries the one state the notifications package has no word for: a match that
// names another submission.
var rn4States = map[notifications.EmailState]rn4MatchState{
	notifications.EmailStateSent:    rn4Sent,
	notifications.EmailStateUnsent:  rn4Unsent,
	notifications.EmailStateUnknown: rn4Unknown,
}

// matchNamesSubmission reports whether the event behind a match names the
// submission. It answers false for ok when the event cannot be read.
func (l PocketBaseDeliveryLookup) matchNamesSubmission(match *core.Record, submissionID string) (mine bool, ok bool) {
	event, err := l.App.FindRecordById(collections.Events, match.GetString("event"))
	if err != nil {
		return false, false
	}
	var payload struct {
		SubmissionID string `json:"submissionId"`
	}
	if json.Unmarshal([]byte(event.GetString("payload")), &payload) != nil {
		return false, false
	}
	return payload.SubmissionID == submissionID, true
}
