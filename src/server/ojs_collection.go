package main

import (
	"context"
	"errors"
	"log"
	"server/internal/audit"
	"server/internal/editorial"
)

type editorialRunner interface {
	RunCycle(context.Context, editorial.RunSource, string, bool) (editorial.Result, error)
}

type scheduler interface{ MustAdd(string, string, func()) }

func registerCollectionJob(s scheduler, expression string, runner editorialRunner) {
	s.MustAdd("ojs-collection", expression, func() {
		if _, err := runner.RunCycle(context.Background(), editorial.RunSourceScheduled, "", false); err != nil && !errors.Is(err, editorial.ErrRunConflict) {
			log.Printf("OJS collection failed: %s", audit.RedactError(err.Error()))
		}
	})
}
