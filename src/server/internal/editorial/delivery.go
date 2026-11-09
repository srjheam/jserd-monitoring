package editorial

import (
	"context"
	"fmt"
	"server/internal/collections"
	"server/internal/email"
	"sync"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// DeliveryService sends the editorial summary e-mail for one cycle, at most
// once. delivery_pipeline.go holds the render-and-send steps it runs,
// pending_articles.go holds the article list it sends, and
// delivery_attempts.go holds the claim/lease bookkeeping that makes one send
// per cycle safe under concurrent callers.
type DeliveryService struct {
	App        core.App
	Resolver   RecipientResolver
	Renderer   Renderer
	Transport  email.Transport
	From       string
	BeforeSend func()
	OwnerID    string
	mu         sync.Mutex
}

var ErrAmbiguousDelivery = fmt.Errorf("editorial delivery has an ambiguous sending claim")

func (s *DeliveryService) Deliver(ctx context.Context, cycleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fresh, err := s.isFirstAttempt(cycleID)
	if err != nil || !fresh {
		return err
	}
	return s.deliverCycle(ctx, cycleID)
}

// isFirstAttempt reports whether this cycle was never delivered before. A
// delivery already marked "sending" is ambiguous: another sender holds the
// claim on it.
func (s *DeliveryService) isFirstAttempt(cycleID string) (bool, error) {
	attempts, err := s.App.FindRecordsByFilter(collections.EditorialDeliveryAttempts, "cycle = {:cycle}", "", 1, 0, dbx.Params{"cycle": cycleID})
	if err != nil {
		return false, err
	}
	if len(attempts) == 0 {
		return true, nil
	}
	if attempts[0].GetString("status") == "sending" {
		return false, ErrAmbiguousDelivery
	}
	return false, nil
}
