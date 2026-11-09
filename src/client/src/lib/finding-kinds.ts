// Generated from src/server/internal/editorial/catalog.go. Do not edit.
// Regenerate with:
//   go test ./internal/editorial -run TestGeneratedFindingKindsFile -update
//
// The order is the rank, worst first. The client does not sort by it: the
// server already ships each submission's findings in this order. The list
// exists so a test can assert that every kind has a translated label.
export const findingKinds = [
  "missing_editor",
  "missing_copyeditor",
  "unsent_review_completion",
  "reviews_complete_no_decision",
  "revision_pending_decision",
  "missing_revision_indication",
  "overdue_review",
  "missing_reviewer",
  "overdue_review_invitation",
  "awaiting_desk_decision",
] as const;

export type FindingKind = (typeof findingKinds)[number];
