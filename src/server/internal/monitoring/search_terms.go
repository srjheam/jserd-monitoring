package monitoring

import "strings"

// parseSearchQuery reads the search wire form
// `editor:<name> folder:<slug> free words editor:<name>` that the work list
// sends. Every `editor:` token becomes an editor chip, and everything else
// stays free text. An editor name that carries spaces is quoted, as
// `editor:"Editor Name"`, because the system has no editor id: the
// display name is the only identity an editor has today.
//
// At most one `folder:` token is expected; the slug it names is returned
// alongside the chips and the free text, unchanged. This function only
// rejects a slug no queue answers to, because that is a client mistake and
// has to surface as HTTP 400. parseScope in queue.go resolves the slug, so
// the `work`/`worklist`/`daily` alias is applied once, there.
func parseSearchQuery(raw string) (chips []string, folder string, free string, err error) {
	tokens := splitSearchTokens(raw)
	folder, err = searchFolder(tokens)
	if err != nil {
		return nil, "", "", err
	}
	chips, free = searchChipsAndText(tokens)
	return chips, folder, free, nil
}

// tokenValue returns the value a prefixed search token carries, or an empty
// string when the token does not carry that prefix, or carries it with nothing
// after it.
func tokenValue(token, prefix string) string {
	value, ok := strings.CutPrefix(token, prefix)
	if !ok {
		return ""
	}
	return value
}

// searchFolder returns the queue slug the query text names, or an empty string
// when it names no folder. The last folder: token wins.
func searchFolder(tokens []string) (string, error) {
	folder := ""
	for _, token := range tokens {
		slug := tokenValue(token, "folder:")
		if slug == "" {
			continue
		}
		if _, known := folderQueue(slug); !known {
			return "", clientError{"unknown folder: " + slug}
		}
		folder = slug
	}
	return folder, nil
}

// searchChipsAndText splits the tokens into the editor chips and the free text.
// A folder: token belongs to neither, because searchFolder already read it.
func searchChipsAndText(tokens []string) (chips []string, free string) {
	chips = []string{}
	words := []string{}
	for _, token := range tokens {
		if name := tokenValue(token, "editor:"); name != "" {
			chips = append(chips, name)
			continue
		}
		if tokenValue(token, "folder:") != "" {
			continue
		}
		words = append(words, token)
	}
	return chips, strings.Join(words, " ")
}

// folderQueue reports whether a client folder slug names a queue scope, and
// passes the slug through unchanged. The queue module resolves the aliases, so
// there is nothing to translate here: this only rejects an unknown slug, which
// is a client error and has to surface as HTTP 400.
func folderQueue(slug string) (string, bool) {
	if _, ok := parseScope(slug); !ok {
		return "", false
	}
	return slug, true
}

// splitSearchTokens splits on whitespace, except inside double quotes, and
// drops the quotes it consumed.
func splitSearchTokens(raw string) []string {
	tokens := []string{}
	current := strings.Builder{}
	quoted := false
	for _, symbol := range raw {
		switch {
		case symbol == '"':
			quoted = !quoted
		case separatesTokens(symbol, quoted):
			tokens = appendToken(tokens, &current)
		default:
			current.WriteRune(symbol)
		}
	}
	return appendToken(tokens, &current)
}

// separatesTokens reports whether a symbol ends the search token being built.
// Whitespace inside double quotes belongs to the token.
func separatesTokens(symbol rune, quoted bool) bool {
	return !quoted && strings.ContainsRune(" \t\n", symbol)
}

// appendToken adds the token built so far and resets the builder. An empty
// builder adds nothing, so a run of separators yields one token, not many.
func appendToken(tokens []string, current *strings.Builder) []string {
	if current.Len() == 0 {
		return tokens
	}
	tokens = append(tokens, current.String())
	current.Reset()
	return tokens
}
