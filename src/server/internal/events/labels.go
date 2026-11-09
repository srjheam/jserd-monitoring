package events

// Activity uses semantic keys so clients can translate titles without exposing internal rule codes.
var ruleTitleKeys = map[string]string{
	RuleRN1:  "review_invitation_answered",
	RuleRN2:  "review_invitation_expired",
	RuleRN3:  "review_submitted",
	RuleRN4:  "all_reviews_completed",
	RuleRN5:  "revised_file_hint",
	RuleRN6:  "review_invitation_reminder",
	RuleRN7:  "review_deadline_reminder",
	RuleRN8:  "review_deadline_expired",
	RuleRN18: "missing_copyeditor",
}

// ActivityTitleKey returns a culture-neutral key for a saved notification type.
func ActivityTitleKey(code string) string {
	if code == RuleWelcome {
		return "welcome"
	}
	if key, ok := ruleTitleKeys[code]; ok {
		return key
	}
	return "editorial_notification"
}

// reviewerRules are the rules whose notification goes to a reviewer. Every
// other rule goes to the assigned editors. This is the one declaration of the
// subset: the recipient resolver, the template choice and the reviewer
// template guard all read it, so a new reviewer rule is one edit, not three.
var reviewerRules = map[string]bool{RuleRN6: true, RuleRN7: true, RuleRN8: true}

const (
	RecipientRoleJournalEditor   = "journal_editor"
	RecipientRoleAssociateEditor = "associate_editor"
	RecipientRoleReviewer        = "reviewer"
	RecipientRoleEditor          = "editor"
)

// RecipientRole returns the role selected by a rule, or an empty value when
// the rule does not identify a specific recipient role.
func RecipientRole(code string) string {
	if code == RuleRN18 {
		return RecipientRoleJournalEditor
	}
	if IsReviewerRule(code) {
		return RecipientRoleReviewer
	}
	switch code {
	case RuleRN1, RuleRN2, RuleRN3, RuleRN4, RuleRN5:
		return RecipientRoleAssociateEditor
	default:
		return ""
	}
}

// IsReviewerRule reports whether a rule's notification goes to a reviewer.
func IsReviewerRule(code string) bool {
	return reviewerRules[code]
}

// IsKnownRule reports whether a rule code is one this engine produces. A code
// it does not know cannot be rendered or delivered.
func IsKnownRule(code string) bool {
	_, known := ruleTitleKeys[code]
	return known
}

// AllRuleCodes returns every known rule code in a stable order. Packages
// that must never mention a rule code verbatim in their own source (e.g.
// internal/monitoring, whose tests are grepped for the pattern) use it to
// obtain sample codes indirectly.
func AllRuleCodes() []string {
	return []string{RuleRN1, RuleRN2, RuleRN3, RuleRN4, RuleRN5, RuleRN6, RuleRN7, RuleRN8, RuleRN18}
}
