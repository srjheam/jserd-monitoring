package editorial

import (
	"context"
	"server/internal/collections"
	"sort"

	"github.com/pocketbase/pocketbase/core"
)

type ChiefEditor struct {
	Name, Email string
}

type ChiefEditorGroup struct {
	ID, Name string
	Specific bool
	Members  []ChiefEditor
}

type ChiefEditorSource interface {
	Groups(context.Context) ([]ChiefEditorGroup, error)
}

type Recipient struct {
	Name, Address string
}

type RecipientResult struct {
	Valid   []Recipient
	Blocked []ChiefEditor
}

// RecipientResolver names the chief editors an editorial summary e-mail
// goes to. recipient_group_match.go holds the OJS-group lookup candidates
// tries first, and recipient_validation.go holds the address validation
// that sorts each candidate into RecipientResult's two lists.
type RecipientResolver struct {
	App       core.App
	OJS       ChiefEditorSource
	GroupID   string
	GroupName string
}

func (r RecipientResolver) Resolve(ctx context.Context) (RecipientResult, error) {
	candidates, err := r.candidates(ctx)
	if err != nil {
		return RecipientResult{}, err
	}
	result := RecipientResult{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		addResolvedRecipient(&result, seen, candidate)
	}
	sort.Slice(result.Valid, func(i, j int) bool { return result.Valid[i].Address < result.Valid[j].Address })
	return result, nil
}

// candidates are the chief editors to send to: the members of the named OJS
// group when OJS answers with exactly one such group, and the stored recipient
// list otherwise.
func (r RecipientResolver) candidates(ctx context.Context) ([]ChiefEditor, error) {
	if candidates, useOJS := r.ojsCandidates(ctx); useOJS {
		return candidates, nil
	}
	return r.storedCandidates()
}

// storedCandidates are the active rows of the stored recipient list.
func (r RecipientResolver) storedCandidates() ([]ChiefEditor, error) {
	records, err := r.App.FindRecordsByFilter(collections.EditorialRecipients, "active = true", "email", 0, 0)
	if err != nil {
		return nil, err
	}
	candidates := make([]ChiefEditor, 0, len(records))
	for _, record := range records {
		candidates = append(candidates, ChiefEditor{Name: record.GetString("name"), Email: record.GetString("email")})
	}
	return candidates, nil
}
