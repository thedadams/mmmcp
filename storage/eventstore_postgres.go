package storage

import (
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
)

func isPostgresRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "40001" || pgErr.Code == "40P01"
}

func intString(value int) string {
	return strconv.Itoa(value)
}
