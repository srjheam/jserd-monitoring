// Package dberr reads the SQLite driver's errors. It knows how
// modernc.org/sqlite wraps and codes an error, and nothing about the
// collections the rest of the server stores.
package dberr

import (
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// constraintCodeMask keeps the primary result code and drops the extended
// half. SQLite returns SQLITE_CONSTRAINT_UNIQUE and its siblings in the high
// bits, and every one of them is a constraint.
const constraintCodeMask = 0xff

// IsConstraint reports whether a save failed because it broke a constraint,
// which is how this server reads "somebody else got there first". Four packages
// claim a row by writing a unique key and treating the loss as a normal
// outcome: state, editorial, email, and notifications.
func IsConstraint(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code()&constraintCodeMask == sqlite3.SQLITE_CONSTRAINT
}
