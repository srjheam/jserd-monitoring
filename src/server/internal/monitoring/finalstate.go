package monitoring

import "strings"

func effectiveFinal(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "published":
		return "accepted"
	case "declined", "rejected":
		return "rejected"
	case "withdrawn":
		return "withdrawn"
	}
	return ""
}
