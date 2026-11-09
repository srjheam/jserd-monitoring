package normalizer

import (
	"server/internal/audit"
	"time"
)

type ArtifactRef struct {
	Kind      string
	PathOrURL string
}

type SourceField struct {
	Source   string
	Column   string
	Artifact string
	Value    string
}

type FieldProvenance = SourceField

type NormalizedCycle struct {
	SourceMode  string
	CollectedAt time.Time
	Submissions []Submission
	Reviews     []ReviewAssignment
	ViewMetrics []ViewMetric
	Artifacts   []audit.RawArtifact
}

type Submission struct {
	OJSSubmissionID string
	Title           string
	AbstractText    string
	Status          string
	URL             string
	DOI             string
	DateSubmitted   string
	LastModified    string
	// DateLastActivity is OJS's own last-activity timestamp, read from the
	// REST submission list. LastModified comes from the article export CSV and
	// tracks a different, unrelated kind of edit, so the two are kept apart.
	DateLastActivity string
	// RevisionPending is OJS's own review-round state for whether the
	// author's revision is still outstanding (see server/internal/ojs,
	// SubmissionDetail.LatestRoundRevisionPending). Nil means not observed:
	// only a live per-submission detail fetch can populate it, so it stays
	// nil for a submission that never became an RN5/RN11 candidate.
	RevisionPending *bool
	// CopyeditorAssigned is OJS's observed assignment state for the
	// Copyediting stage. Nil means no assignment evidence was collected.
	CopyeditorAssigned *bool
	// ActiveReviewers is OJS's live count of the latest round's reviewers who
	// still staff it (see ojs.SubmissionDetail.ActiveReviewers). The Reviews
	// export cannot tell a decline from an acceptance, so RN9 and RN19 read
	// this count when it is set. Nil means not observed.
	ActiveReviewers    *int
	SubmissionProgress int
	Authors            []Author
	Editors            []Editor
	Provenance         map[string]FieldProvenance
}

type Author struct {
	Sequence     int
	GivenName    string
	FamilyName   string
	ORCID        string
	Country      string
	Affiliation  string
	Email        string
	HomepageURL  string
	BioStatement string
}

type Editor struct {
	Sequence   int
	GivenName  string
	FamilyName string
	ORCID      string
	Email      string
	Decisions  []EditorDecision
}

type EditorDecision struct {
	Sequence    int
	Decision    string
	DateDecided string
}

type ReviewAssignment struct {
	Key                 string
	Stage               string
	Round               int
	SubmissionTitle     string
	SubmissionID        string
	ReviewerUsername    string
	ReviewerGivenName   string
	ReviewerFamilyName  string
	ReviewerEmail       string
	DateAssigned        string
	DateNotified        string
	DateConfirmed       string
	DateCompleted       string
	DateAcknowledged    string
	DateReminded        string
	ResponseDueDate     string
	ResponseOverdueDays int
	ReviewDueDate       string
	ReviewOverdueDays   int
	Declined            bool
	Unconsidered        bool
	Provenance          map[string]FieldProvenance

	// RoundHasOverdueResponse is set by a pre-Process enrichment stage from
	// OJS's own reviewAssignment statusId (RESPONSE_OVERDUE), not the CSV
	// export's Declined column, which is always "Yes" regardless of true
	// state. It is round-level, not
	// reviewer-level: OJS's REST API has no way to join a reviewAssignment
	// back to a specific reviewer identity, so every review row sharing a
	// submission and round gets the same value.
	RoundHasOverdueResponse bool

	// ReviewDueWindowSuperseded is set by a second pre-Process enrichment
	// stage from OJS's own reviewAssignment statusId, keyed by (round,
	// review due date). It is true when OJS's live state no longer treats
	// this window as a pending review: every live reviewAssignment sharing
	// the round and due date is submitted, completed, thanked, or
	// cancelled, or no live reviewAssignment matches the window at all —
	// the CSV export's row is stale. Like
	// RoundHasOverdueResponse, this cannot be joined to a specific
	// reviewer identity, so it is round+due level, not reviewer-level.
	ReviewDueWindowSuperseded bool
}

type ViewMetric struct {
	ArticleID        string
	Title            string
	Issue            string
	DatePublished    string
	AbstractViews    int
	TotalGalleyViews int
	GalleyViews      map[string]int
	Provenance       map[string]FieldProvenance
}
