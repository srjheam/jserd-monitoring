package normalizer

import "time"

// ojsDateLayouts are every shape OJS reports a date in, across the CSV exports
// and the REST API. They are listed here once: three packages read these same
// strings, and a layout missing from one of them silently drops a date that
// the others accept.
//
// A PocketBase timestamp is not in this list and does not belong here. Read
// one with record.GetDateTime instead of parsing its text.
var ojsDateLayouts = []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}

// ParseDate reads an OJS date string. It reports false when no known layout
// matches, which includes the empty string.
func ParseDate(value string) (time.Time, bool) {
	for _, layout := range ojsDateLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}
