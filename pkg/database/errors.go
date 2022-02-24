package database

import (
	"database/sql"
	"github.com/lib/pq"
)

type Error struct {
	Err error
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func (e *Error) DuplicateKey() bool {
	// unique_violation
	if pgerr, ok := e.Err.(*pq.Error); ok && pgerr.Code == "23505" {
		return true
	}

	return false
}

func (e *Error) RowNotFound() bool {
	return e.Err == sql.ErrNoRows
}
