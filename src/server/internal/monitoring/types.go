// Package monitoring exposes a safe, authenticated editorial monitoring projection.
package monitoring

import "time"

const Route = "/api/jserd/monitoring"

type State string

const (
	StateAvailable   State = "available"
	StateStale       State = "stale"
	StateEmpty       State = "empty"
	StateUnavailable State = "unavailable"
)

type Provenance map[string]string

// Meta is the monitoring overview served without the paginated submission
// rows; it carries cycle metadata, conferences and precomputed counts
// that previously required shipping every submission to the client.
type Meta struct {
	GeneratedAt   time.Time    `json:"generatedAt"`
	Availability  State        `json:"availability"`
	LatestAttempt *Collection  `json:"latestAttempt,omitempty"`
	LatestSuccess *Collection  `json:"latestSuccess,omitempty"`
	Conferences   []Conference `json:"conferences"`
	Aggregates    Aggregates   `json:"aggregates"`
}

type Aggregates struct {
	Metrics map[string]int `json:"metrics"`
	// Triage counts the submissions in each queue, keyed by queue name (see
	// queue.go). It is a map rather than one field per queue so that adding a
	// queue is one edit to `queues` instead of a Go field, a TypeScript field
	// and a new switch arm.
	Triage map[string]int `json:"triage"`
	// DailyRequiringAttention is not a queue, so it does not live in Triage.
	DailyRequiringAttention int               `json:"dailyRequiringAttention"`
	Statuses                []string          `json:"statuses"`
	Editors                 []EditorFrequency `json:"editors"`
	Total                   int               `json:"total"`
}

type Collection struct {
	ID         string    `json:"id"`
	Source     string    `json:"source"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	Failure    string    `json:"failure,omitempty"`
}
type Conference struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type Decision struct {
	Decision string `json:"decision"`
	Date     string `json:"date"`
}
type Review struct {
	Round            int    `json:"round"`
	Status           string `json:"status"`
	ResponseDeadline string `json:"responseDeadline"`
	ReviewDeadline   string `json:"reviewDeadline"`
}
type Finding struct {
	Kind        string `json:"kind"`
	Responsible string `json:"responsible"`
	Deadline    string `json:"deadline"`
}
type LocalTracking struct {
	ID             string `json:"id"`
	StatusDetail   string `json:"statusDetail"`
	EditorNote     string `json:"editorNote"`
	LastVerifiedAt string `json:"lastVerifiedAt"`
	VerifiedBy     string `json:"verifiedBy"`
	Observations   string `json:"observations"`
	DecisionDetail string `json:"decisionDetail"`
	Conference     string `json:"conference"`
	Muted          bool   `json:"muted"`
	MutedDigest    string `json:"mutedDigest"`
	UpToDateAt     string `json:"upToDateAt"`
	UpToDateDigest string `json:"upToDateDigest"`
	UpToDateUntil  string `json:"upToDateUntil"`
	// UpToDateUntilAt is the same return date, read from the DateField rather
	// than parsed back out of UpToDateUntil above. A PocketBase timestamp is
	// not an OJS date, and parsing one as text is what made the three OJS date
	// parsers drift apart. It stays off the wire: the client reads the string.
	UpToDateUntilAt time.Time `json:"-"`
	Created         string    `json:"created"`
	Updated         string    `json:"updated"`
}
type Submission struct {
	OJSSubmissionID     string         `json:"ojsSubmissionId"`
	Title               string         `json:"title"`
	Status              string         `json:"status"`
	URL                 string         `json:"url"`
	DateSubmitted       string         `json:"dateSubmitted"`
	LastModified        string         `json:"lastModified"`
	DateLastActivity    string         `json:"dateLastActivity"`
	Editors             []string       `json:"editors"`
	Decisions           []Decision     `json:"decisions"`
	CopyeditorAssigned  *bool          `json:"copyeditorAssigned,omitempty"`
	Reviews             []Review       `json:"reviews"`
	Findings            []Finding      `json:"findings"`
	Tracking            *LocalTracking `json:"tracking,omitempty"`
	EffectiveFinalState string         `json:"effectiveFinalState"`
	EffectiveQueue      string         `json:"effectiveQueue"`
	EffectiveAttention  string         `json:"effectiveAttention"`
	// FindingDigest is the stable hash of the current finding kinds. The work
	// list stores it as up_to_date_digest when a chief editor marks a
	// submission up to date, so the marking ends by itself once the findings
	// change.
	FindingDigest string     `json:"findingDigest"`
	Provenance    Provenance `json:"provenance,omitempty"`
}
