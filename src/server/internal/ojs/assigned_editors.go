package ojs

import (
	"context"
	"fmt"
	"server/internal/normalizer"
	"strings"

	"golang.org/x/net/html"
)

// AssignedEditors returns the normalized names assigned to a submission group.
func (c *Client) AssignedEditors(ctx context.Context, submissionID string, stageID int, groupLabel string) ([]string, error) {
	gridURL, err := c.participantGridURL(submissionID, stageID)
	if err != nil {
		return nil, err
	}
	content, err := c.fetchParticipantGrid(ctx, gridURL)
	if err != nil {
		return nil, err
	}
	return parseAssignedEditorsGrid(content, groupLabel)
}

func parseAssignedEditorsGrid(markup, groupLabel string) ([]string, error) {
	root, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return nil, fmt.Errorf("OJS participant grid markup is invalid")
	}
	grid, ok := participantGridRoot(root)
	if !ok {
		return nil, fmt.Errorf("OJS participant grid markup is invalid")
	}
	categories := map[string]string{}
	participants := map[string][]string{}
	collectParticipantCategories(grid, categories, participants)
	if len(categories) == 0 && len(participants) == 0 && hasVisibleEmptyState(grid) {
		return nil, nil
	}
	if err := validateParticipantCategories(categories, participants); err != nil {
		return nil, err
	}
	wanted := normalizeParticipantGroupLabel(groupLabel)
	var names []string
	for categoryID, label := range categories {
		if label == wanted {
			names = append(names, participants[categoryID]...)
		}
	}
	return normalizedParticipantNames(names)
}

func normalizedParticipantNames(names []string) ([]string, error) {
	seen := map[string]bool{}
	var normalized []string
	for _, name := range names {
		slug := normalizer.NameSlug(name)
		if slug == "" {
			return nil, fmt.Errorf("OJS participant grid participant name is missing")
		}
		if !seen[slug] {
			seen[slug] = true
			normalized = append(normalized, slug)
		}
	}
	return normalized, nil
}

func normalizeParticipantGroupLabel(label string) string {
	return strings.Join(strings.Fields(strings.ToLower(label)), " ")
}
