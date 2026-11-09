package state

import (
	"context"
	"errors"
	"server/internal/collections"

	"github.com/pocketbase/pocketbase/core"
)

var errImmutableOperationalRecord = errors.New("operational records are immutable")

// retentionUpdateKey marks a context as an authorized retention-prune update
// (clearing an old snapshot's payload to free disk). It is unexported so
// only code in this package can set it - every other caller (admin UI, API)
// still gets a blanket rejection.
type retentionUpdateKey struct{}

func withRetentionUpdate(ctx context.Context) context.Context {
	return context.WithValue(ctx, retentionUpdateKey{}, true)
}

func isRetentionUpdate(ctx context.Context) bool {
	allowed, _ := ctx.Value(retentionUpdateKey{}).(bool)
	return allowed
}

func RegisterImmutableHooks(app core.App) {
	strict := []string{
		collections.CollectionCycles, collections.Events, collections.EventRuleMatches,
		collections.Notifications, collections.NotificationTargets, collections.EmailDeliveryRequests,
		collections.EmailDeliveryClaims, collections.EmailDeliveryAttempts, collections.SubmissionJournal,
	}
	app.OnRecordUpdateExecute(strict...).BindFunc(func(*core.RecordEvent) error {
		return errImmutableOperationalRecord
	})
	app.OnRecordUpdateExecute(collections.Snapshots).BindFunc(func(e *core.RecordEvent) error {
		if isRetentionUpdate(e.Context) {
			return e.Next()
		}
		return errImmutableOperationalRecord
	})
	app.OnRecordDeleteExecute(append([]string{collections.Snapshots}, strict...)...).BindFunc(func(*core.RecordEvent) error {
		return errImmutableOperationalRecord
	})
}
