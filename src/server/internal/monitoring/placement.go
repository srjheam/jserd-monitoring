package monitoring

import (
	"server/internal/editorial"
	"strings"
	"time"
)

// queueFor resolves where a submission belongs: archived for a final status
// or confirmed Copyeditor assignment during Copyediting, Muted while the chief editor has
// delegated it, Up to date once nothing needs a chief editor, or Daily
// otherwise. A submission with no current findings is Up to date without a chief editor action;
// otherwise it is Up to date only while the tracking record's stored digest
// still matches the submission's current finding digest and any return date
// has not passed. An assigned conference does not change this: it is a
// label, not a placement of its own.
func queueFor(status, finalState string, copyeditorAssigned *bool, tracking *LocalTracking, findings []editorial.Finding) string {
	if finalState != "" {
		return queueArchived
	}
	if strings.EqualFold(strings.TrimSpace(status), "copyediting") && copyeditorAssigned != nil && *copyeditorAssigned {
		return queueArchived
	}
	if isMuted(tracking) {
		return queueMuted
	}
	if needsNoChiefEditor(tracking, findings) {
		return queueUpToDate
	}
	return defaultQueue
}

// needsNoChiefEditor reports whether the submission needs nothing from a chief
// editor now: it carries no finding, or a chief editor set it aside and that
// set-aside still holds.
func needsNoChiefEditor(tracking *LocalTracking, findings []editorial.Finding) bool {
	if len(findings) == 0 {
		return true
	}
	return setAsideHolds(tracking, findings)
}

// setAsideHolds reports whether the set-aside a chief editor made still holds:
// the findings have not changed since, and the return date has not passed.
func setAsideHolds(tracking *LocalTracking, findings []editorial.Finding) bool {
	if tracking == nil || tracking.UpToDateDigest == "" {
		return false
	}
	return !upToDateUntilPassed(tracking.UpToDateUntilAt) && tracking.UpToDateDigest == editorial.DigestFor(findings)
}

// upToDateUntilPassed reads the return date a chief editor chose. A zero time
// means no return date was set, which never expires.
func upToDateUntilPassed(until time.Time) bool {
	return !until.IsZero() && !until.After(time.Now().UTC())
}
