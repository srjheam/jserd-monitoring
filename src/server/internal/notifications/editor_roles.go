package notifications

import (
	"context"
	"fmt"
	"server/internal/events"
	"slices"
)

// EditorTargets reads the group selected for this submission's stage assignment.
type EditorTargets func(context.Context, string, string) ([]string, error)

type EditorAssignmentSource interface {
	Login(context.Context, string, string) error
	AssignedEditors(context.Context, string, int, string) ([]string, error)
}

const (
	reviewStageID      = 3
	copyeditingStageID = 4
)

type OJSEditorTargets struct {
	Client             EditorAssignmentSource
	Username, Password string
}

func (s OJSEditorTargets) Keys(ctx context.Context, rule, submissionID string) ([]string, error) {
	stage, group := reviewStageID, "Associate Editor"
	if events.RecipientRole(rule) == events.RecipientRoleJournalEditor {
		stage, group = copyeditingStageID, "Journal editor"
	}
	if s.Client == nil {
		return nil, fmt.Errorf("editor assignment source is unavailable")
	}
	if err := s.Client.Login(ctx, s.Username, s.Password); err != nil {
		return nil, err
	}
	return s.Client.AssignedEditors(ctx, submissionID, stage, group)
}

func (s *Service) editorKeysForMatch(matchID string) ([]string, error) {
	match, rule, err := supportedMatch(s.App, matchID)
	if err != nil || events.IsReviewerRule(rule) {
		return nil, err
	}
	_, payload, err := eventPayload(s.App, match)
	if err != nil {
		return nil, err
	}
	if s.EditorTargets == nil {
		return nil, fmt.Errorf("editor assignment source is unavailable")
	}
	assigned, err := s.EditorTargets(context.Background(), rule, payload.SubmissionID)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	for _, key := range payload.EditorTargetKeys {
		if slices.Contains(assigned, key) {
			keys = append(keys, key)
		}
	}
	return keys, nil
}
