package editorial

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"server/internal/collections"
	"server/internal/email"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// This file holds DeliveryService's render-and-send steps, from the
// pending-article list pending_articles.go builds through to the SMTP send
// and the attempt record delivery_attempts.go persists.

// deliverCycle sends the editorial summary e-mail for one cycle. A cycle whose
// articles all need nothing from a chief editor sends no e-mail at all.
func (s *DeliveryService) deliverCycle(ctx context.Context, cycleID string) error {
	articles, err := s.pendingArticles()
	if err != nil || len(articles) == 0 {
		return err
	}
	return s.renderAndSend(ctx, cycleID, articles)
}

// renderAndSend resolves the chief editors, then renders and sends the summary.
func (s *DeliveryService) renderAndSend(ctx context.Context, cycleID string, articles []ArticleFinding) error {
	cycle, err := s.App.FindRecordById(collections.CollectionCycles, cycleID)
	if err != nil {
		return err
	}
	resolved, err := s.Resolver.Resolve(ctx)
	if err != nil {
		return err
	}
	// The point in time the findings it reports were current as of, matching
	// CONTEXT.md's Editorial summary e-mail entry: findings are live, so the
	// e-mail carries no date range for them to summarize.
	period := cycle.GetDateTime("finished_at").Time().UTC().Format("2006-01-02 15:04")
	return s.sendSummary(ctx, cycleID, period, resolved, articles)
}

// sendSummary renders the summary and sends it. A render failure, and a run
// with no valid recipient, each end as a recorded attempt instead of a retry.
func (s *DeliveryService) sendSummary(ctx context.Context, cycleID, period string, resolved RecipientResult, articles []ArticleFinding) error {
	rendered, err := s.Renderer.Render(period, articles)
	if err != nil {
		// A placeholder for a hash that was never computed, because rendering
		// itself failed. The width is read off the hash function, not spelled.
		placeholder := RenderedSummary{Version: SummaryTemplateVersion, Hash: strings.Repeat("0", hex.EncodedLen(sha256.Size))}
		return s.saveAttempt(cycleID, resolved, placeholder, "failed", "render failure")
	}
	if len(resolved.Valid) == 0 {
		return s.saveAttempt(cycleID, resolved, rendered, "blocked", "no valid chief editor recipient")
	}
	return s.claimAndSend(ctx, cycleID, resolved, rendered)
}

// claimAndSend takes the sending claim for this cycle, then sends. Another
// sender that already holds the claim wins, and this call sends nothing.
func (s *DeliveryService) claimAndSend(ctx context.Context, cycleID string, resolved RecipientResult, rendered RenderedSummary) error {
	attempt, claimed, err := s.claimAttempt(cycleID, resolved, rendered)
	if err != nil || !claimed {
		return err
	}
	return s.sendAndFinish(ctx, attempt, resolved, rendered)
}

// sendAndFinish sends the message and records how the send ended.
func (s *DeliveryService) sendAndFinish(ctx context.Context, attempt *core.Record, resolved RecipientResult, rendered RenderedSummary) error {
	if s.BeforeSend != nil {
		s.BeforeSend()
	}
	message := email.Message{To: recipientList(resolved.Valid), From: s.From, Subject: rendered.Subject, Text: rendered.Text, HTML: rendered.HTML}
	if sendErr := s.Transport.Send(ctx, message); sendErr != nil {
		return errors.Join(sendErr, s.finishAttempt(attempt, "failed", "SMTP send failed"))
	}
	return s.finishAttempt(attempt, "sent", "")
}

// recipientList is the To header for a set of chief editors.
func recipientList(recipients []Recipient) string {
	addresses := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		addresses = append(addresses, (&mail.Address{Name: recipient.Name, Address: recipient.Address}).String())
	}
	return strings.Join(addresses, ", ")
}
