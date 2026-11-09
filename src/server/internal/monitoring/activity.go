package monitoring

import (
	"server/internal/collections"
	"server/internal/events"
	"server/internal/pbfilter"
	"sort"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// ActivityItem is one row of the submission activity timeline: either a
// delivered/blocked/failed notification (kind "delivery") or an editor
// journal entry (kind "journal"), merged and sorted by time descending.
type ActivityItem struct {
	Kind          string    `json:"kind"`
	OccurredAt    time.Time `json:"occurredAt"`
	TitleKey      string    `json:"titleKey,omitempty"`
	Status        string    `json:"status,omitempty"`
	FailureClass  string    `json:"failureClass,omitempty"`
	Recipient     string    `json:"recipient,omitempty"`
	RecipientRole string    `json:"recipientRole,omitempty"`
	AuthorLabel   string    `json:"authorLabel,omitempty"`
	Body          string    `json:"body,omitempty"`
}

const (
	activityKindDelivery = "delivery"
	activityKindJournal  = "journal"
)

// FindActivity merges email_delivery_attempts (joined to their originating
// event, to recover the submission id) with submission_journal entries for
// ojsSubmissionID, sorted by time descending.
func FindActivity(app core.App, ojsSubmissionID string) ([]ActivityItem, error) {
	deliveries, err := deliveryActivity(app, ojsSubmissionID)
	if err != nil {
		return nil, err
	}
	entries, err := journalActivity(app, ojsSubmissionID)
	if err != nil {
		return nil, err
	}
	deliveries = append(deliveries, entries...)
	sort.SliceStable(deliveries, func(i, j int) bool { return deliveries[i].OccurredAt.After(deliveries[j].OccurredAt) })
	return deliveries, nil
}

// deliveryActivity lists every notification e-mail sent about the submission.
func deliveryActivity(app core.App, ojsSubmissionID string) ([]ActivityItem, error) {
	eventIDs, err := matchingEventIDs(app, ojsSubmissionID)
	if err != nil || len(eventIDs) == 0 {
		return []ActivityItem{}, err
	}
	attempts, occurredAt, err := timedDeliveryAttempts(app, eventIDs)
	if err != nil {
		return nil, err
	}
	return deliveryItems(attempts, occurredAt), nil
}

// timedDeliveryAttempts lists the delivery attempts the given events caused,
// together with the moment each event occurred.
func timedDeliveryAttempts(app core.App, eventIDs []string) ([]*core.Record, map[string]time.Time, error) {
	occurredAt, err := eventTimes(app, eventIDs)
	if err != nil {
		return nil, nil, err
	}
	filter, params := pbfilter.IDFilter("event", eventIDs)
	attempts, err := app.FindRecordsByFilter(collections.EmailDeliveryAttempts, filter, "", 0, 0, params)
	if err != nil {
		return nil, nil, err
	}
	return attempts, occurredAt, nil
}

// deliveryItems turns delivery attempts into timeline rows, each stamped with
// the time of the event that caused it.
func deliveryItems(attempts []*core.Record, occurredAt map[string]time.Time) []ActivityItem {
	items := make([]ActivityItem, 0, len(attempts))
	for _, attempt := range attempts {
		items = append(items, ActivityItem{
			Kind:          activityKindDelivery,
			OccurredAt:    occurredAt[attempt.GetString("event")],
			TitleKey:      events.ActivityTitleKey(attempt.GetString("rule_code")),
			Status:        attempt.GetString("status"),
			FailureClass:  attempt.GetString("result_class"),
			Recipient:     attempt.GetString("to_address"),
			RecipientRole: activityRecipientRole(attempt),
		})
	}
	return items
}

func activityRecipientRole(attempt *core.Record) string {
	if role := attempt.GetString("recipient_role"); role != "" {
		return role
	}
	if attempt.GetString("target_kind") == "reviewer" {
		return "reviewer"
	}
	if attempt.GetString("target_kind") == events.RecipientRoleEditor {
		return events.RecipientRoleEditor
	}
	return ""
}

// eventTimes maps each event id to the moment the event occurred. A delivery
// attempt carries no time of its own, so it borrows the time of its event.
func eventTimes(app core.App, eventIDs []string) (map[string]time.Time, error) {
	filter, params := pbfilter.IDFilter("id", eventIDs)
	records, err := app.FindRecordsByFilter(collections.Events, filter, "", 0, 0, params)
	if err != nil {
		return nil, err
	}
	occurredAt := make(map[string]time.Time, len(records))
	for _, event := range records {
		occurredAt[event.Id] = event.GetDateTime("occurred_at").Time()
	}
	return occurredAt, nil
}

// journalActivity lists every journal entry a chief editor wrote about the
// submission.
func journalActivity(app core.App, ojsSubmissionID string) ([]ActivityItem, error) {
	records, err := app.FindRecordsByFilter(collections.SubmissionJournal, "ojs_submission_id = {:id}", "-entry_date", 0, 0, dbx.Params{"id": ojsSubmissionID})
	if err != nil {
		return nil, err
	}
	items := make([]ActivityItem, 0, len(records))
	for _, entry := range records {
		items = append(items, ActivityItem{
			Kind:        activityKindJournal,
			OccurredAt:  entry.GetDateTime("entry_date").Time(),
			AuthorLabel: entry.GetString("author_label"),
			Body:        entry.GetString("body"),
		})
	}
	return items, nil
}

// matchingEventIDs returns the ids of every event whose entity_key or
// payload.submissionId is ojsSubmissionID. It is written as a UNION of two
// single-column equality queries, rather than one query with an OR, because
// SQLite can use idx_events_entity_key and idx_events_payload_submission_id
// for each branch of a UNION; an OR combining both columns falls back to a
// full table scan.
func matchingEventIDs(app core.App, ojsSubmissionID string) ([]string, error) {
	var ids []string
	err := app.DB().NewQuery(`
		SELECT id FROM events WHERE entity_key = {:id}
		UNION
		SELECT id FROM events WHERE (CASE WHEN json_valid(payload) THEN JSON_EXTRACT(payload, '$.submissionId') ELSE JSON_EXTRACT(json_object('pb', payload), '$.pbsubmissionId') END) = {:id}
	`).Bind(dbx.Params{"id": ojsSubmissionID}).Column(&ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}
