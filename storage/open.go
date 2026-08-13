package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"  // Register the SQLite database/sql driver.
	_ "github.com/go-sql-driver/mysql" // Register the MySQL database/sql driver.
	_ "github.com/jackc/pgx/v5/stdlib" // Register the PostgreSQL database/sql driver.
	"github.com/obot-platform/mmmcp/storage/migrations"
)

// Open classifies and opens a supported SQL store, verifies it, and applies
// all dialect migrations before returning it to callers.
func Open(ctx context.Context, dsn string, options Options) (*SQLStore, error) {
	options = options.normalized()
	classified, err := classifyDSN(dsn, options)
	if err != nil {
		return nil, err
	}
	if classified.path != "" {
		if err := os.MkdirAll(filepath.Dir(classified.path), 0o700); err != nil {
			return nil, storageError(classified.dialect, "create data directory", err)
		}
	}

	db, err := sql.Open(classified.driver, classified.value)
	if err != nil {
		return nil, storageError(classified.dialect, "open", err)
	}
	if classified.dialect == DialectSQLite {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	} else {
		db.SetMaxOpenConns(options.MaxOpenConns)
		db.SetMaxIdleConns(options.MaxIdleConns)
		db.SetConnMaxLifetime(options.ConnMaxLifetime)
	}
	closeOnError := func(operation string, cause error) (*SQLStore, error) {
		_ = db.Close()
		return nil, storageError(classified.dialect, operation, cause)
	}
	if err := db.PingContext(ctx); err != nil {
		return closeOnError("ping", err)
	}
	if err := migrations.Up(ctx, db, migrationDialect(classified.dialect)); err != nil {
		return closeOnError("migrate", err)
	}
	store := &SQLStore{db: db, dialect: classified.dialect, options: options}
	if err := store.Prune(ctx); err != nil {
		return closeOnError("prune", err)
	}
	return store, nil
}

func migrationDialect(dialect Dialect) migrations.Dialect {
	switch dialect {
	case DialectPostgres:
		return migrations.Postgres
	case DialectMySQL:
		return migrations.MySQL
	default:
		return migrations.SQLite
	}
}
