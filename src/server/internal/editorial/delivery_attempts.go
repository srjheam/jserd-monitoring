package editorial

import (
	"encoding/json"
	"fmt"
	"os"
	"server/internal/collections"
	"server/internal/dberr"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// This file persists one editorial delivery attempt per send outcome
// (failed, blocked, sent), and the claim/lease pair that stops two
// concurrent senders from both sending the same cycle's summary.

// newAttemptRecord builds one editorial delivery attempt record.
func (s *DeliveryService) newAttemptRecord(cycleID string, recipients RecipientResult, rendered RenderedSummary) (*core.Record, error) {
	collection, err := s.App.FindCollectionByNameOrId(collections.EditorialDeliveryAttempts)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(recipients)
	if err != nil {
		return nil, err
	}
	record := core.NewRecord(collection)
	record.Set("cycle", cycleID)
	record.Set("recipients", payload)
	record.Set("subject", rendered.Subject)
	record.Set("template_version", rendered.Version)
	record.Set("template_hash", rendered.Hash)
	return record, nil
}

// leaseOwner names the sender that holds the claim.
func (s *DeliveryService) leaseOwner() string {
	if s.OwnerID != "" {
		return s.OwnerID
	}
	return "editorial-delivery-" + strconv.Itoa(os.Getpid())
}

func (s *DeliveryService) saveAttempt(cycleID string, recipients RecipientResult, rendered RenderedSummary, status, sanitizedError string) error {
	record, err := s.newAttemptRecord(cycleID, recipients, rendered)
	if err != nil {
		return err
	}
	record.Set("status", status)
	record.Set("sanitized_error", sanitizedError)
	if err := s.App.Save(record); err != nil {
		if dberr.IsConstraint(err) {
			return nil
		}
		return fmt.Errorf("save editorial delivery attempt: %w", err)
	}
	return nil
}

func (s *DeliveryService) claimAttempt(cycleID string, recipients RecipientResult, rendered RenderedSummary) (*core.Record, bool, error) {
	record, err := s.newAttemptRecord(cycleID, recipients, rendered)
	if err != nil {
		return nil, false, err
	}
	// How long a claim protects a sending attempt from a concurrent sender
	// before another caller may retry it.
	const leaseDuration = 15 * time.Minute
	record.Set("status", "sending")
	record.Set("lease_owner", s.leaseOwner())
	record.Set("lease_expires_at", time.Now().UTC().Add(leaseDuration))
	if err := s.App.Save(record); err != nil {
		if dberr.IsConstraint(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("claim editorial delivery: %w", err)
	}
	return record, true, nil
}

func (s *DeliveryService) finishAttempt(record *core.Record, status, sanitizedError string) error {
	record.Set("status", status)
	record.Set("sanitized_error", sanitizedError)
	if err := s.App.Save(record); err != nil {
		return fmt.Errorf("finish editorial delivery attempt: %w", err)
	}
	return nil
}
