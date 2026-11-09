// Package collections names every PocketBase collection this server reads or
// writes.
//
// A collection name is a schema fact, and a rename is a migration. Naming each
// one here makes a rename one edit plus a compiler error at every reader,
// rather than a grep across every package that a single miss passes. A missed
// literal compiles and fails at run time, inside a hook, where the cycle logs
// the error and still reports success.
//
// The migrations package does not use these names, by design. A migration
// records what the schema was at one point in time, so it has to keep the
// literal it ran against, even after a later migration renames the collection.
package collections

const (
	// The collection cycle: one run of the collector, and the snapshot it took.
	CollectionCycles         = "collection_cycles"
	Snapshots                = "snapshots"
	ManualCollectionRequests = "manual_collection_requests"

	// The projection the dashboard reads, and the chief editor's curation of it.
	OJSSubmissions     = "ojs_submissions"
	SubmissionTracking = "submission_tracking"
	SubmissionJournal  = "submission_journal"
	Conferences        = "conferences"

	// The editorial findings, their summary schedule, and the summary e-mails.
	EditorialFindings         = "editorial_findings"
	EditorialSchedule         = "editorial_schedule"
	EditorialRecipients       = "editorial_recipients"
	EditorialDeliveryAttempts = "editorial_delivery_attempts"

	// The notification engine: an event, the rules it matched, and the
	// notifications and e-mails those matches produced.
	Events                = "events"
	EventRuleMatches      = "event_rule_matches"
	Notifications         = "notifications"
	NotificationTargets   = "notification_targets"
	Recipients            = "recipients"
	EmailDeliveryRequests = "email_delivery_requests"
	EmailDeliveryClaims   = "email_delivery_claims"
	EmailDeliveryAttempts = "email_delivery_attempts"

	// ChiefEditors is the auth collection: a chief editor's dashboard sign-in
	// account. It is not the e-mail roster, which is EditorialRecipients above
	// and carried this name until migration 1788900000.
	ChiefEditors = "chief_editors"
)
