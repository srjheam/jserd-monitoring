package ojs

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

func parseCopyeditorGrid(markup string) (bool, error) {
	root, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return false, fmt.Errorf("OJS participant grid markup is invalid")
	}
	grid, ok := participantGridRoot(root)
	if !ok {
		return false, fmt.Errorf("OJS participant grid markup is invalid")
	}
	categories := map[string]string{}
	participantCategories := map[string][]string{}
	collectParticipantCategories(grid, categories, participantCategories)
	if len(categories) == 0 && len(participantCategories) == 0 && hasVisibleEmptyState(grid) {
		return false, nil
	}
	if err := validateParticipantCategories(categories, participantCategories); err != nil {
		return false, err
	}
	for category, label := range categories {
		if isCopyeditorLabel(label) && len(participantCategories[category]) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func validateParticipantCategories(categories map[string]string, participants map[string][]string) error {
	if len(categories) == 0 {
		return fmt.Errorf("OJS participant grid has no categories")
	}
	for category, label := range categories {
		if label == "" {
			return fmt.Errorf("OJS participant grid category label is missing")
		}
		if len(participants[category]) == 0 {
			return fmt.Errorf("OJS participant grid category has no participants")
		}
	}
	for category := range participants {
		if _, ok := categories[category]; !ok {
			return fmt.Errorf("OJS participant grid participant has no category")
		}
	}
	return nil
}

func isCopyeditorLabel(label string) bool {
	return label == "copyeditor" || label == "editor de texto"
}
