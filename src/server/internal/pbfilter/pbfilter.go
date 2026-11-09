// Package pbfilter builds PocketBase filter expressions that the record
// finders take. It knows about PocketBase query syntax and nothing about OJS.
package pbfilter

import (
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
)

// IDFilter builds a filter expression matching any of the given values for
// field, along with the params it references. An empty ids list produces an
// empty expression, which PocketBase reads as "no filter": callers that mean
// "match nothing" must check the length themselves.
func IDFilter(field string, ids []string) (string, dbx.Params) {
	clauses := make([]string, len(ids))
	params := dbx.Params{}
	for i, id := range ids {
		key := fmt.Sprintf("idf%d", i)
		clauses[i] = field + " = {:" + key + "}"
		params[key] = id
	}
	return strings.Join(clauses, " || "), params
}
