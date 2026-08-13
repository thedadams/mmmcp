// Package migrations embeds and applies storage schema migrations.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"

	"github.com/pressly/goose/v3"
)

//go:embed sqlite/*.sql postgres/*.sql mysql/*.sql
var files embed.FS

// Dialect identifies one embedded migration set.
type Dialect string

const (
	SQLite   Dialect = "sqlite"
	Postgres Dialect = "postgres"
	MySQL    Dialect = "mysql"
)

// Up applies all pending migrations for dialect. Omitting dialect preserves
// the original SQLite-only API.
func Up(ctx context.Context, db *sql.DB, selected ...Dialect) error {
	dialect := SQLite
	if len(selected) > 0 {
		dialect = selected[0]
	}
	if len(selected) > 1 {
		return fs.ErrInvalid
	}
	dialectFiles, err := fs.Sub(files, string(dialect))
	if err != nil {
		return err
	}
	gooseDialect := goose.DialectSQLite3
	switch dialect {
	case SQLite:
	case Postgres:
		gooseDialect = goose.DialectPostgres
	case MySQL:
		gooseDialect = goose.DialectMySQL
	default:
		return fs.ErrInvalid
	}
	provider, err := goose.NewProvider(gooseDialect, db, dialectFiles)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}
