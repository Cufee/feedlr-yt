package database

import (
	"database/sql"
	"errors"
)

func IsErrNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, sql.ErrNoRows)
}
