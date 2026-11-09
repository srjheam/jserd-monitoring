package email

import (
	"fmt"
	"server/internal/collections"
	"server/internal/dberr"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// newClaimRecord builds one delivery claim row.
func (w *Worker) newClaimRecord(app core.App, request *core.Record) (*core.Record, error) {
	collection, err := app.FindCollectionByNameOrId(collections.EmailDeliveryClaims)
	if err != nil {
		return nil, err
	}
	claim := core.NewRecord(collection)
	claim.Set("request", request.Id)
	claim.Set("claimed_at", time.Now().UTC())
	claim.Set("owner_id", w.OwnerID)
	return claim, nil
}

// saveClaimRecord writes the claim, through the writer the caller configured.
func (w *Worker) saveClaimRecord(app core.App, claim *core.Record) error {
	if w.BeforeClaim != nil {
		w.BeforeClaim()
	}
	if w.SaveClaim != nil {
		return w.SaveClaim(app, claim)
	}
	return app.Save(claim)
}

// resolveClaimConflict accepts the claim another worker took first. A failure
// that is not a unique-key conflict is reported as it is.
func (w *Worker) resolveClaimConflict(app core.App, request *core.Record, saveErr error) error {
	if !dberr.IsConstraint(saveErr) {
		return saveErr
	}
	held, err := w.claimHeld(app, request)
	if err != nil || !held {
		return fmt.Errorf("claim constraint without matching request: %w", saveErr)
	}
	return nil
}
