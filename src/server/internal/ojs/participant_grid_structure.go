package ojs

import (
	"strings"

	"golang.org/x/net/html"
)

const participantGridIDPrefix = "component-grid-users-stageparticipant-stageparticipantgrid-"

func participantGridRoot(node *html.Node) (*html.Node, bool) {
	gridID := attribute(node, "id")
	candidate := node.Type == html.ElementNode && node.Data == "div" &&
		hasClass(attribute(node, "class"), "pkp_controllers_grid") &&
		strings.HasPrefix(gridID, participantGridIDPrefix) && len(gridID) > len(participantGridIDPrefix)
	if candidate && tableByID(node, gridID+"-table") {
		return node, true
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if grid, ok := participantGridRoot(child); ok {
			return grid, true
		}
	}
	return nil, false
}

func tableByID(node *html.Node, tableID string) bool {
	if node.Type == html.ElementNode && node.Data == "table" && attribute(node, "id") == tableID {
		return true
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if tableByID(child, tableID) {
			return true
		}
	}
	return false
}
