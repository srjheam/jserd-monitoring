package editorial

import (
	"encoding/json"
	"fmt"
	"server/internal/collections"
	"sync"

	"github.com/pocketbase/pocketbase/core"
)

// findingPayload is the JSON shape stored in editorial_findings.payload:
// everything about a submission's findings that nothing filters or sorts on,
// bundled together instead of split into columns.
type findingPayload struct {
	Title    string    `json:"title"`
	URL      string    `json:"url"`
	Findings []Finding `json:"findings"`
}

type Store struct {
	App core.App
	mu  sync.Mutex
}

func NewStore(app core.App) *Store { return &Store{App: app} }

// ReplaceFindings replaces the entire editorial_findings table with the
// findings produced by the cycle cycleID: delete-all, insert-fresh, the same
// pattern ojs_submissions already uses. It is called once per cycle, right
// after finding evaluation, so editorial_findings always holds the latest
// cycle's findings and nothing older. cycleID is accepted for symmetry with
// the coordinator's call, not stored: editorial_findings holds no run,
// window, or cycle bookkeeping, only the latest state.
func (s *Store) ReplaceFindings(cycleID string, findings []ArticleFinding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	collection, err := s.App.FindCollectionByNameOrId(collections.EditorialFindings)
	if err != nil {
		return fmt.Errorf("find editorial_findings: %w", err)
	}
	return s.App.RunInTransaction(func(app core.App) error {
		return storeFindings(app, collection, findings)
	})
}

// storeFindings clears the findings table and writes the given findings in.
func storeFindings(app core.App, collection *core.Collection, findings []ArticleFinding) error {
	if _, err := app.DB().NewQuery("DELETE FROM editorial_findings").Execute(); err != nil {
		return fmt.Errorf("clear editorial_findings: %w", err)
	}
	for _, finding := range findings {
		if err := storeFinding(app, collection, finding); err != nil {
			return err
		}
	}
	return nil
}

// storeFinding writes the findings of one submission.
func storeFinding(app core.App, collection *core.Collection, finding ArticleFinding) error {
	payload, err := json.Marshal(findingPayload{Title: finding.Title, URL: finding.URL, Findings: finding.Findings})
	if err != nil {
		return fmt.Errorf("marshal findings: %w", err)
	}
	record := core.NewRecord(collection)
	record.Set("submission_key", finding.SubmissionID)
	record.Set("date_submitted", finding.DateSubmitted)
	record.Set("payload", payload)
	return app.Save(record)
}
