package editorial

import (
	"context"
	"server/internal/ojs"
	"sync"
)

// SubmissionDetailSource is the OJS surface a SubmissionDetailLoader needs:
// re-authenticate, then fetch one submission's detail record. *ojs.Client
// satisfies this.
type SubmissionDetailSource interface {
	Login(ctx context.Context, username, password string) error
	SubmissionDetail(ctx context.Context, submissionID string) (ojs.SubmissionDetail, error)
}

// SubmissionDetailProvider is what an enrichment stage depends on.
// *SubmissionDetailLoader satisfies it; tests use a lightweight fake.
type SubmissionDetailProvider interface {
	Get(ctx context.Context, submissionID string) (ojs.SubmissionDetail, error)
}

// SubmissionDetailLoader fetches GET submissions/{id} at most once per
// submission per cycle, so RN5, RN2, and RN7/RN8's enrichment stages can
// share one live detail fetch for a submission that is a candidate for more
// than one of them in the same cycle, instead of each issuing its own
// request. One loader is built fresh
// per Coordinator.RunCycle call and discarded after — it is not a
// long-lived cache.
type SubmissionDetailLoader struct {
	client   SubmissionDetailSource
	username string
	password string
	mu       sync.Mutex
	loggedIn bool
	cache    map[string]ojs.SubmissionDetail
}

func NewSubmissionDetailLoader(client SubmissionDetailSource, username, password string) *SubmissionDetailLoader {
	return &SubmissionDetailLoader{client: client, username: username, password: password, cache: map[string]ojs.SubmissionDetail{}}
}

// Get returns submissionID's detail record, fetching it live only on the
// first call for that id within this loader's lifetime. A failed fetch is
// not cached, so a later Get for the same id retries instead of replaying
// the failure.
func (l *SubmissionDetailLoader) Get(ctx context.Context, submissionID string) (ojs.SubmissionDetail, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if detail, ok := l.cache[submissionID]; ok {
		return detail, nil
	}
	if err := l.ensureLoggedIn(ctx); err != nil {
		return ojs.SubmissionDetail{}, err
	}
	detail, err := l.client.SubmissionDetail(ctx, submissionID)
	if err != nil {
		return ojs.SubmissionDetail{}, err
	}
	l.cache[submissionID] = detail
	return detail, nil
}

func (l *SubmissionDetailLoader) ensureLoggedIn(ctx context.Context) error {
	if l.loggedIn {
		return nil
	}
	if err := l.client.Login(ctx, l.username, l.password); err != nil {
		return err
	}
	l.loggedIn = true
	return nil
}
