package ojs

import (
	"strings"

	"golang.org/x/net/html"
)

func findLabel(node *html.Node) string {
	if node.Type == html.ElementNode && node.Data == "span" && hasClass(attribute(node, "class"), "label") {
		return nodeText(node)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if label := findLabel(child); label != "" {
			return label
		}
	}
	return ""
}

func nodeText(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	var text strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		text.WriteString(nodeText(child))
	}
	return text.String()
}

func attribute(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func hasClass(classes, target string) bool {
	for _, class := range strings.Fields(classes) {
		if class == target {
			return true
		}
	}
	return false
}
