package monitoring

import (
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
)

// The queue names, as the effective_queue column stores them. queueFor in
// placement.go returns one of these and nothing else.
const (
	queueUpToDate = "up_to_date"
	queueMuted    = "muted"
	queueArchived = "archived"
)

// queue is one work queue: the name the effective_queue column stores, and the
// slug the client spells it with in a URL and in a `folder:` chip.
type queue struct {
	name string
	slug string
}

// queues is the one list of queues, in display order, worst first. Adding a
// queue here gives it a scope, a sort position, an aggregate count and a client
// slug together. queueFor in placement.go returns one of these names, and
// src/client/src/lib/wire.ts is generated from this list.
var queues = []queue{
	{name: defaultQueue, slug: "work"},
	{name: queueUpToDate, slug: queueUpToDate},
	{name: queueMuted, slug: queueMuted},
	{name: queueArchived, slug: queueArchived},
}

// queueNames returns the queue names in display order.
func queueNames() []string {
	names := make([]string, 0, len(queues))
	for _, entry := range queues {
		names = append(names, entry.name)
	}
	return names
}

// emptyTriage returns a zeroed count for every queue. Every queue reports a
// number even when it holds nothing, so the client never has to tell "no
// submissions" apart from "this queue does not exist".
func emptyTriage() map[string]int {
	counts := make(map[string]int, len(queues))
	for _, entry := range queues {
		counts[entry.name] = 0
	}
	return counts
}

// scope is a resolved queue selection: which submissions a request is asking
// about. The zero value is unrestricted.
type queueScope struct {
	// queue names the single queue to match, or "" when the scope is not a
	// single queue.
	queue string
	// excludeArchived is the "active" scope: everything still open.
	excludeArchived bool
}

// parseScope reads a client folder slug and returns the scope it selects.
//
// `work`, `worklist` and `daily` are three spellings of one queue: the client
// says `work` in a URL and a `folder:` chip, the list route has long accepted
// `worklist`, and the column stores `daily`. The alias lives here alone.
func parseScope(slug string) (queueScope, bool) {
	if resolved, ok := scopeAliases[slug]; ok {
		return resolved, true
	}
	return namedQueue(slug)
}

// scopeAliases holds every slug that names no single queue, plus `worklist`,
// the spelling the list route accepted before the client said `work`. A slug
// absent from here has to match a queue name or a queue slug to be accepted.
var scopeAliases = map[string]queueScope{
	"":         {},
	"all":      {},
	"active":   {excludeArchived: true},
	"worklist": {queue: defaultQueue},
}

// namedQueue matches a slug against the stored queue names and the client
// slugs. Both spellings select the same queue.
func namedQueue(slug string) (queueScope, bool) {
	for _, entry := range queues {
		if slug == entry.name || slug == entry.slug {
			return queueScope{queue: entry.name}, true
		}
	}
	return queueScope{}, false
}

// mustScope is parseScope for a queue name this package produces itself, where
// an unknown value is a programming error rather than bad input.
func mustScope(slug string) queueScope {
	resolved, ok := parseScope(slug)
	if !ok {
		panic("monitoring: unknown queue scope " + slug)
	}
	return resolved
}

// predicate returns the SQL condition matching this scope, and the params it
// references. An unrestricted scope returns an empty condition, so a caller
// can skip the clause instead of writing "1 = 1".
//
// The condition names the `s` alias, which every caller uses for
// ojs_submissions.
func (s queueScope) predicate() (string, dbx.Params) {
	switch {
	case s.excludeArchived:
		return "s.effective_queue <> '" + queueArchived + "'", dbx.Params{}
	case s.queue != "":
		return "s.effective_queue = {:queue}", dbx.Params{"queue": s.queue}
	default:
		return "", dbx.Params{}
	}
}

// orderRank folds the queue's display order into one sortable digit, so the
// status a submission is in outranks whichever column the chief editor picked
// to sort by. A digit is inverted when that column sorts descending, so the
// group order survives sharing one ORDER BY direction with it.
func orderRank(desc bool) string {
	cases := make([]string, 0, len(queues))
	for i, entry := range queues {
		cases = append(cases, fmt.Sprintf("WHEN '%s' THEN '%d'", entry.name, rankDigit(i, desc)))
	}
	return "(CASE s.effective_queue " + strings.Join(cases, " ") + " ELSE '" + fmt.Sprintf("%d", rankDigit(len(queues), desc)) + "' END)"
}

// maxRankDigit is the highest single digit the folded sort key can carry.
// rankDigit must stay a single character for every real index, including
// the one past the last queue that orderRank's ELSE branch also folds in.
const maxRankDigit = 9

// rankDirections maps the descending flag to the base and the step
// rankDigit walks from it, so a rank never branches on the flag directly.
var rankDirections = map[bool]struct{ base, step int }{
	false: {base: 0, step: 1},
	true:  {base: maxRankDigit, step: -1},
}

func rankDigit(index int, desc bool) int {
	direction := rankDirections[desc]
	return direction.base + direction.step*index
}
