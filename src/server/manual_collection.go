package main

import (
	"context"
	"errors"
	"fmt"
	"server/internal/audit"
	"server/internal/collections"
	"server/internal/editorial"

	"github.com/pocketbase/pocketbase/core"
)

func startupRecovery(run func()) { go run() }

func validateManualRequest(record *core.Record, operator string) error {
	if !record.GetBool("confirmed") {
		return errors.New("manual collection requires explicit confirmation")
	}
	if operator == "" {
		return errors.New("manual collection requires a superuser operator")
	}
	record.Set("operator", operator)
	record.Set("status", "pending")
	return nil
}

func registerManualCollectionHooks(app core.App, runner editorialRunner) {
	app.OnRecordCreateRequest(collections.ManualCollectionRequests).BindFunc(checkManualRequest)
	app.OnRecordAfterCreateSuccess(collections.ManualCollectionRequests).BindFunc(func(e *core.RecordEvent) error {
		return startManualRequest(e, app, runner)
	})
}

// checkManualRequest accepts a manual collection request from a Superuser only.
func checkManualRequest(event *core.RecordRequestEvent) error {
	info, err := event.RequestInfo()
	if err != nil || !info.HasSuperuserAuth() {
		return errors.New("manual collection requires superuser authentication")
	}
	if err := validateManualRequest(event.Record, info.Auth.Id); err != nil {
		return err
	}
	return event.Next()
}

// startManualRequest runs one manual collection request in the background.
func startManualRequest(event *core.RecordEvent, app core.App, runner editorialRunner) error {
	requestID := event.Record.Id
	startupRecovery(func() {
		if err := executeManualRequest(context.Background(), app, requestID, runner); err != nil {
			app.Logger().Error("manual collection request failed", "request", requestID, "error", audit.RedactError(err.Error()))
		}
	})
	return event.Next()
}

func executeManualRequest(ctx context.Context, app core.App, requestID string, runner editorialRunner) error {
	record, err := claimManualRequest(app, requestID)
	if err != nil || record == nil {
		return err
	}
	result, runErr := runner.RunCycle(ctx, editorial.RunSourceManual, record.Id, record.GetBool("send_digest"))
	recordRunOutcome(record, result, runErr)
	if err := app.Save(record); err != nil {
		return fmt.Errorf("finalize manual collection request: %w", err)
	}
	return nil
}

// claimManualRequest marks one pending, confirmed request as running. It
// answers nil for a request that is not ready to run.
func claimManualRequest(app core.App, requestID string) (*core.Record, error) {
	record, err := app.FindRecordById(collections.ManualCollectionRequests, requestID)
	if err != nil {
		return nil, fmt.Errorf("load manual collection request: %w", err)
	}
	if !readyToRun(record) {
		return nil, nil
	}
	record.Set("status", "running")
	if err := app.Save(record); err != nil {
		return nil, fmt.Errorf("mark manual collection request running: %w", err)
	}
	return record, nil
}

// readyToRun reports whether a manual collection request waits to run, and a
// Superuser confirmed it.
func readyToRun(record *core.Record) bool {
	return record.GetString("status") == "pending" && record.GetBool("confirmed")
}

// recordRunOutcome stamps how one manual collection request ended.
func recordRunOutcome(record *core.Record, result editorial.Result, runErr error) {
	if result.CycleID != "" {
		record.Set("cycle", result.CycleID)
	}
	if runErr != nil {
		record.Set("status", "failed")
		record.Set("sanitized_error", audit.RedactError(runErr.Error()))
		return
	}
	record.Set("status", "succeeded")
	record.Set("sanitized_error", "")
}
