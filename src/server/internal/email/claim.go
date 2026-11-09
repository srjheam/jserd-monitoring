package email

import (
	"server/internal/collections"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// claimForDelivery reports whether this worker owns the delivery of one
// request: the request was never attempted before, and this worker took the
// claim on it.
func (w *Worker) claimForDelivery(request *core.Record) (bool, error) {
	attempted, err := w.alreadyAttempted(request)
	if err != nil || attempted {
		return false, err
	}
	claimed := false
	err = w.App.RunInTransaction(func(app core.App) error {
		var txErr error
		claimed, txErr = w.writeClaim(app, request)
		return txErr
	})
	return claimed, err
}

// alreadyAttempted reports whether this request was already sent, blocked or
// failed once.
func (w *Worker) alreadyAttempted(request *core.Record) (bool, error) {
	attempts, err := w.App.FindRecordsByFilter(collections.EmailDeliveryAttempts, "request = {:id}", "", 1, 0, dbx.Params{"id": request.Id})
	if err != nil {
		return false, err
	}
	return len(attempts) > 0, nil
}

// writeClaim inserts the claim row. A unique-key conflict means that another
// worker claimed the request first, and that worker wins.
func (w *Worker) writeClaim(app core.App, request *core.Record) (bool, error) {
	claim, err := w.pendingClaim(app, request)
	if err != nil || claim == nil {
		return false, err
	}
	if err := w.saveClaimRecord(app, claim); err != nil {
		return false, w.resolveClaimConflict(app, request, err)
	}
	return true, nil
}

// pendingClaim is the claim row to write for one request, or nil when a claim
// on that request already exists.
func (w *Worker) pendingClaim(app core.App, request *core.Record) (*core.Record, error) {
	held, err := w.claimHeld(app, request)
	if err != nil || held {
		return nil, err
	}
	return w.newClaimRecord(app, request)
}

// claimHeld reports whether a claim on this request already exists.
func (w *Worker) claimHeld(app core.App, request *core.Record) (bool, error) {
	existing, err := app.FindRecordsByFilter(collections.EmailDeliveryClaims, "request = {:id}", "", 1, 0, dbx.Params{"id": request.Id})
	if err != nil {
		return false, err
	}
	return len(existing) > 0, nil
}
