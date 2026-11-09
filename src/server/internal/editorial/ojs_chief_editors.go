package editorial

import (
	"context"
	"server/internal/ojs"
	"strings"
)

type OJSChiefEditorSource struct {
	Client    *ojs.Client
	Username  string
	Password  string
	GroupID   string
	GroupName string
}

func (s OJSChiefEditorSource) Groups(ctx context.Context) ([]ChiefEditorGroup, error) {
	if err := s.Client.Login(ctx, s.Username, s.Password); err != nil {
		return nil, err
	}
	users, err := s.Client.ListUserGroups(ctx)
	if err != nil {
		return nil, err
	}
	return s.groupMembers(users), nil
}

// groupMembers gathers every OJS group the users belong to, in the order OJS
// first reported each group.
func (s OJSChiefEditorSource) groupMembers(users []ojs.UserSummary) []ChiefEditorGroup {
	groups := map[string]*ChiefEditorGroup{}
	order := []string{}
	for _, user := range users {
		for _, group := range user.Groups {
			order = s.addGroupMember(groups, order, group, user)
		}
	}
	result := make([]ChiefEditorGroup, 0, len(order))
	for _, key := range order {
		result = append(result, *groups[key])
	}
	return result
}

// addGroupMember records one user as a member of one OJS group, creating the
// group on first sight. It returns the group order with the new group appended,
// so the result keeps the order OJS reported the groups in.
func (s OJSChiefEditorSource) addGroupMember(groups map[string]*ChiefEditorGroup, order []string, group ojs.UserGroup, user ojs.UserSummary) []string {
	key := group.ID + "\x00" + group.Name
	entry := groups[key]
	if entry == nil {
		entry = &ChiefEditorGroup{ID: group.ID, Name: group.Name, Specific: s.isConfiguredGroup(group)}
		groups[key] = entry
		order = append(order, key)
	}
	entry.Members = append(entry.Members, ChiefEditor{Name: user.Name, Email: user.Email})
	return order
}

// isConfiguredGroup reports whether an OJS group is the one this source was
// configured to read, by id or by name.
func (s OJSChiefEditorSource) isConfiguredGroup(group ojs.UserGroup) bool {
	byID := s.GroupID != "" && group.ID == s.GroupID
	byName := s.GroupName != "" && strings.EqualFold(strings.TrimSpace(group.Name), strings.TrimSpace(s.GroupName))
	return byID || byName
}
