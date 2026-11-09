package main

import "server/internal/collector"

type cycleProcessor interface {
	ProcessCycle(collector.CycleResult) error
}

type pendingCreator interface {
	CreatePending() error
}

func processCollectedCycle(store cycleProcessor, notifications pendingCreator, result collector.CycleResult) (error, error) {
	if err := store.ProcessCycle(result); err != nil {
		return err, nil
	}
	return nil, notifications.CreatePending()
}
