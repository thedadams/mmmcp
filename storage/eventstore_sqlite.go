package storage

import (
	"context"
	"database/sql"
	"errors"

	glebarezsqlite "github.com/glebarez/go-sqlite"
)

// Prune removes expired streams and enough of the oldest event payloads to
// enforce the configured size bound. The newest event is retained even when a
// single payload is larger than the bound.
func (s *SQLStore) Prune(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	err := s.runTransaction(ctx, nil, func(tx *sql.Tx) error {
		return s.pruneTx(ctx, tx, s.options.Now().UnixNano())
	})
	if err != nil {
		return storageError(s.dialect, "prune events", err)
	}
	return nil
}

func (s *SQLStore) pruneTx(ctx context.Context, tx *sql.Tx, now int64) error {
	if _, err := tx.ExecContext(ctx, s.bind(`
		DELETE FROM mmmcp_event_log
		WHERE EXISTS (
			SELECT 1 FROM mmmcp_event_streams streams
			WHERE streams.session_id = mmmcp_event_log.session_id
			  AND streams.stream_id = mmmcp_event_log.stream_id
			  AND streams.expires_at <= ?
		)`), now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, s.bind(`DELETE FROM mmmcp_event_streams WHERE expires_at <= ?`), now); err != nil {
		return err
	}

	var totalBytes, eventCount int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(payload_size), 0), COUNT(*)
		FROM mmmcp_event_log`).Scan(&totalBytes, &eventCount); err != nil {
		return err
	}
	for totalBytes > s.options.MaxEventBytes && eventCount > 1 {
		var sessionID, streamID string
		var index, payloadSize int64
		selectOldest := `
			SELECT session_id, stream_id, event_index, payload_size
			FROM mmmcp_event_log
			ORDER BY created_at, session_id, stream_id, event_index
			LIMIT 1`
		if s.dialect != DialectSQLite {
			selectOldest += " FOR UPDATE"
		}
		if err := tx.QueryRowContext(ctx, selectOldest).Scan(&sessionID, &streamID, &index, &payloadSize); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, s.bind(`
			DELETE FROM mmmcp_event_log
			WHERE session_id = ? AND stream_id = ? AND event_index = ?`),
			sessionID, streamID, index); err != nil {
			return err
		}
		watermark := index + 1
		if _, err := tx.ExecContext(ctx, s.bind(`
			UPDATE mmmcp_event_streams
			SET first_retained_index = CASE
				WHEN first_retained_index < ? THEN ?
				ELSE first_retained_index
			END
			WHERE session_id = ? AND stream_id = ?`),
			watermark, watermark, sessionID, streamID); err != nil {
			return err
		}
		totalBytes -= payloadSize
		eventCount--
	}
	return nil
}

func isSQLiteRetryable(err error) bool {
	var sqliteErr *glebarezsqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	switch sqliteErr.Code() & 0xff {
	case 5, 6: // SQLITE_BUSY and SQLITE_LOCKED, including extended codes.
		return true
	default:
		return false
	}
}
