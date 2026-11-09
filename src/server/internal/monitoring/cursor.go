package monitoring

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
)

// cursorPayload is the decoded form of a keyset pagination cursor: the value
// and id of the last row on the previous page, and the sort it was issued
// for.
type cursorPayload struct {
	V string `json:"v"`
	I string `json:"i"`
	K string `json:"k"`
	D string `json:"d"`
}

func encodeCursor(payload cursorPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeCursor(raw string) (cursorPayload, error) {
	decoded := cursorPayload{}
	bytes, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return decoded, clientError{"invalid pagination cursor"}
	}
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		return decoded, clientError{"invalid pagination cursor"}
	}
	return decoded, nil
}

// cursorMatchesSort reports whether a cursor was issued for the sort the
// request asks for. The cursor value only compares against that sort.
func cursorMatchesSort(cursor cursorPayload, parsed listQuery) bool {
	return cursor.K == parsed.sortKey && cursor.D == directionLabel(parsed.desc)
}

// applyCursor reads the keyset cursor.
func applyCursor(query url.Values, parsed *listQuery) error {
	raw := query.Get("cursor")
	if raw == "" {
		return nil
	}
	cursor, err := decodeCursor(raw)
	if err != nil {
		return err
	}
	if !cursorMatchesSort(cursor, *parsed) {
		return clientError{"cursor does not match the requested sort"}
	}
	parsed.hasCursor = true
	parsed.cursor = cursor
	return nil
}

func cursorCondition(q listQuery, column string) string {
	comparison := ">"
	if q.desc {
		comparison = "<"
	}
	return "(" + column + " COLLATE NOCASE " + comparison + " {:cursor_value} OR (" + column + " COLLATE NOCASE = {:cursor_value} AND s.ojs_submission_id COLLATE NOCASE " + comparison + " {:cursor_id}))"
}

// nextPageCursor is the cursor that resumes after the last row of a page that
// has more rows to follow. The caller decides whether a next page exists;
// this only encodes the cursor for it.
func nextPageCursor(q listQuery, rows []pageRow) (string, error) {
	last := rows[len(rows)-1]
	return encodeCursor(cursorPayload{V: last.SortKey, I: last.OjsSubmissionID, K: q.sortKey, D: directionLabel(q.desc)})
}

// directionLabels are the two `dir` spellings a cursor's D field carries, so
// a decoded cursor can be compared against the descending flag a listQuery
// resolved.
var directionLabels = map[bool]string{false: "asc", true: "desc"}

func directionLabel(desc bool) string {
	return directionLabels[desc]
}
