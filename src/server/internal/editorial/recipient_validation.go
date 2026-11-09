package editorial

import (
	"net/mail"
	"strings"
)

// This file sorts a candidate chief editor into RecipientResolver.Resolve's
// valid or blocked list.

// addResolvedRecipient sorts one candidate into the valid list or the blocked
// list. A candidate is blocked when its name or address carries a line break,
// when its address is not a bare address, or when an earlier candidate already
// took that address.
func addResolvedRecipient(result *RecipientResult, seen map[string]bool, candidate ChiefEditor) {
	address, ok := bareAddress(candidate)
	if !ok || seen[strings.ToLower(address)] {
		result.Blocked = append(result.Blocked, candidate)
		return
	}
	seen[strings.ToLower(address)] = true
	result.Valid = append(result.Valid, Recipient{Name: candidate.Name, Address: address})
}

// bareAddress is the candidate's e-mail, when it is a plain address with no
// display name and no line break folded into it.
func bareAddress(candidate ChiefEditor) (string, bool) {
	if strings.ContainsAny(candidate.Name+candidate.Email, "\r\n") {
		return "", false
	}
	parsed, err := mail.ParseAddress(candidate.Email)
	if err != nil || parsed.Address != candidate.Email {
		return "", false
	}
	return parsed.Address, true
}
