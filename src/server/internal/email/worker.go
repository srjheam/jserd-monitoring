package email

import (
	"context"
	"errors"
	"server/internal/collections"
	"server/internal/notifications"

	"github.com/pocketbase/pocketbase/core"
)

type Worker struct {
	App           core.App
	Transport     Transport
	Renderer      Renderer
	OwnerID, From string
	ReplyTo       []string
	EditorTargets notifications.EditorTargets
	BeforeClaim   func()
	BeforeSend    func()
	SaveClaim     func(core.App, *core.Record) error
}

type JobScheduler interface {
	MustAdd(jobID, expression string, run func())
}

func RegisterJobs(scheduler JobScheduler, collectionCron, emailCron string, collect, runDue func()) {
	scheduler.MustAdd("ojs-collection", collectionCron, collect)
	scheduler.MustAdd("email-delivery-worker", emailCron, runDue)
}

func (w *Worker) RunDue(ctx context.Context) error {
	requests, err := w.App.FindRecordsByFilter(collections.EmailDeliveryRequests, "", "id", 0, 0)
	if err != nil {
		return err
	}
	var errs []error
	for _, request := range requests {
		if err := w.runRequest(ctx, request); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// runRequest delivers one pending request. A request that another worker
// already attempted, or already claimed, is left alone.
func (w *Worker) runRequest(ctx context.Context, request *core.Record) error {
	attempted, err := w.alreadyAttempted(request)
	if err != nil || attempted {
		return err
	}
	records, allowed, err := w.editorRoleAllowed(ctx, request)
	if err != nil {
		return err
	}
	mine, err := w.claimForDelivery(request)
	if err != nil || !mine {
		return err
	}
	if !allowed {
		outcome := attemptOutcome{status: attemptStatusBlocked, resultClass: "recipient_blocked", message: "recipient is not assigned to the required editor group"}
		return w.saveAttempt(request, records, sentEmail{rendered: blankRendered(), toAddress: records.recipient.GetString("email")}, outcome)
	}
	return w.deliver(ctx, request, records)
}
