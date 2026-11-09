package normalizer

import (
	"strings"
	"unicode"
)

// NameSlug is the client-visible identity of an assigned editor (ADR-0006):
// the display name, lowercased, stripped of punctuation, and hyphenated. Two
// different raw names can produce the same slug by design.
func NameSlug(name string) string {
	cleaned := strings.Map(slugRune, strings.ToLower(name))
	return strings.Join(strings.Fields(cleaned), "-")
}

// slugRune maps one symbol to its slug form. A letter or a digit stays as it is.
func slugRune(symbol rune) rune {
	if unicode.IsLetter(symbol) || unicode.IsDigit(symbol) {
		return symbol
	}
	return slugSeparator(symbol)
}

// slugSeparator maps a word break to a space, and drops every other symbol.
func slugSeparator(symbol rune) rune {
	if symbol == ' ' || symbol == '-' {
		return ' '
	}
	return -1
}
