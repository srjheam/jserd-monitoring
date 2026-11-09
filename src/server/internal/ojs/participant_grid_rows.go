package ojs

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var participantCategoryRowID = regexp.MustCompile(`-category-(\d+)-row-(\d+)$`)

const participantGridRowIDPrefix = "component-grid-users-stageparticipant-stageparticipantgrid-category-"

func hasVisibleEmptyState(node *html.Node) bool {
	// PKP hides this global empty row when it renders any category body.
	if node.Type == html.ElementNode && node.Data == "tbody" &&
		hasClass(attribute(node, "class"), "empty") &&
		!hasClass(attribute(node, "class"), "category_placeholder") &&
		!strings.Contains(strings.ReplaceAll(strings.ToLower(attribute(node, "style")), " ", ""), "display:none") {
		return strings.TrimSpace(nodeText(node)) != ""
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if hasVisibleEmptyState(child) {
			return true
		}
	}
	return false
}

func collectParticipantCategories(node *html.Node, categories map[string]string, participants map[string][]string) {
	if node.Type == html.ElementNode && node.Data == "tr" {
		collectParticipantRow(node, categories, participants)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		collectParticipantCategories(child, categories, participants)
	}
}

func collectParticipantRow(row *html.Node, categories map[string]string, participants map[string][]string) {
	id, classes := attribute(row, "id"), attribute(row, "class")
	ids := participantCategoryRowID.FindStringSubmatch(id)
	if !strings.HasPrefix(id, participantGridRowIDPrefix) || len(ids) != 3 || !hasClass(classes, "gridRow") {
		return
	}
	categoryID := ids[1]
	if hasClass(classes, "category") {
		label := strings.Join(strings.Fields(strings.ToLower(findLabel(row))), " ")
		categories[categoryID] = label
		return
	}
	participants[categoryID] = append(participants[categoryID], findLabel(row))
}
