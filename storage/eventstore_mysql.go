package storage

import (
	"errors"

	"github.com/go-sql-driver/mysql"
)

func isMySQLRetryable(err error) bool {
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) {
		return false
	}
	switch mysqlErr.Number {
	case 1205, 1206, 1213:
		return true
	default:
		return false
	}
}
