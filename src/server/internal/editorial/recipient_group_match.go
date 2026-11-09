package editorial

import (
	"context"
	"strings"
)

// This file names the single specific OJS group RecipientResolver.candidates
// prefers, when this resolver is configured to read one.

func (r RecipientResolver) ojsCandidates(ctx context.Context) ([]ChiefEditor, bool) {
	if !r.namesAGroup() {
		return nil, false
	}
	groups, err := r.OJS.Groups(ctx)
	if err != nil {
		return nil, false
	}
	matches := r.matchingGroups(groups)
	if len(matches) != 1 {
		return nil, false
	}
	return matches[0].Members, true
}

// namesAGroup reports whether this resolver was configured to read an OJS
// group at all.
func (r RecipientResolver) namesAGroup() bool {
	return r.OJS != nil && (r.GroupID != "" || r.GroupName != "")
}

// matchingGroups are the specific OJS groups the configured id or name names.
// A group that is not specific never matches, because a generic group carries
// members of every journal.
func (r RecipientResolver) matchingGroups(groups []ChiefEditorGroup) []ChiefEditorGroup {
	matches := make([]ChiefEditorGroup, 0, 1)
	for _, group := range groups {
		if group.Specific && r.namesGroup(group) {
			matches = append(matches, group)
		}
	}
	return matches
}

// namesGroup reports whether the configured id or name names one group.
func (r RecipientResolver) namesGroup(group ChiefEditorGroup) bool {
	idMatches := r.GroupID != "" && group.ID == r.GroupID
	nameMatches := r.GroupName != "" && strings.EqualFold(strings.TrimSpace(group.Name), strings.TrimSpace(r.GroupName))
	return idMatches || nameMatches
}
