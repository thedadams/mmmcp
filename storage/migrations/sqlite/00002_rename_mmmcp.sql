-- +goose Up
CREATE TABLE IF NOT EXISTS mmcp_event_streams (
    session_id TEXT NOT NULL,
    stream_id TEXT NOT NULL,
    first_retained_index INTEGER NOT NULL DEFAULT 0,
    next_index INTEGER NOT NULL DEFAULT 0,
    expires_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (session_id, stream_id)
);

CREATE TABLE IF NOT EXISTS mmcp_event_log (
    session_id TEXT NOT NULL,
    stream_id TEXT NOT NULL,
    event_index INTEGER NOT NULL,
    payload BLOB,
    payload_is_null INTEGER NOT NULL DEFAULT 0,
    payload_size INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (session_id, stream_id, event_index),
    FOREIGN KEY (session_id, stream_id)
        REFERENCES mmcp_event_streams (session_id, stream_id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS mmmcp_event_streams (
    session_id TEXT NOT NULL,
    stream_id TEXT NOT NULL,
    first_retained_index INTEGER NOT NULL DEFAULT 0,
    next_index INTEGER NOT NULL DEFAULT 0,
    expires_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (session_id, stream_id)
);

CREATE TABLE IF NOT EXISTS mmmcp_event_log (
    session_id TEXT NOT NULL,
    stream_id TEXT NOT NULL,
    event_index INTEGER NOT NULL,
    payload BLOB,
    payload_is_null INTEGER NOT NULL DEFAULT 0,
    payload_size INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (session_id, stream_id, event_index),
    FOREIGN KEY (session_id, stream_id)
        REFERENCES mmmcp_event_streams (session_id, stream_id)
        ON DELETE CASCADE
);

INSERT OR IGNORE INTO mmmcp_event_streams (
    session_id, stream_id, first_retained_index, next_index, expires_at, updated_at
)
SELECT session_id, stream_id, first_retained_index, next_index, expires_at, updated_at
FROM mmcp_event_streams;

INSERT OR IGNORE INTO mmmcp_event_log (
    session_id, stream_id, event_index, payload, payload_is_null, payload_size, created_at
)
SELECT session_id, stream_id, event_index, payload, payload_is_null, payload_size, created_at
FROM mmcp_event_log;

DROP TABLE mmcp_event_log;
DROP TABLE mmcp_event_streams;

CREATE INDEX IF NOT EXISTS mmmcp_event_streams_expiry_idx
    ON mmmcp_event_streams (expires_at);

CREATE INDEX IF NOT EXISTS mmmcp_event_log_created_idx
    ON mmmcp_event_log (created_at, session_id, stream_id, event_index);

-- +goose Down
CREATE TABLE IF NOT EXISTS mmcp_event_streams (
    session_id TEXT NOT NULL,
    stream_id TEXT NOT NULL,
    first_retained_index INTEGER NOT NULL DEFAULT 0,
    next_index INTEGER NOT NULL DEFAULT 0,
    expires_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (session_id, stream_id)
);

CREATE TABLE IF NOT EXISTS mmcp_event_log (
    session_id TEXT NOT NULL,
    stream_id TEXT NOT NULL,
    event_index INTEGER NOT NULL,
    payload BLOB,
    payload_is_null INTEGER NOT NULL DEFAULT 0,
    payload_size INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (session_id, stream_id, event_index),
    FOREIGN KEY (session_id, stream_id)
        REFERENCES mmcp_event_streams (session_id, stream_id)
        ON DELETE CASCADE
);

INSERT OR IGNORE INTO mmcp_event_streams (
    session_id, stream_id, first_retained_index, next_index, expires_at, updated_at
)
SELECT session_id, stream_id, first_retained_index, next_index, expires_at, updated_at
FROM mmmcp_event_streams;

INSERT OR IGNORE INTO mmcp_event_log (
    session_id, stream_id, event_index, payload, payload_is_null, payload_size, created_at
)
SELECT session_id, stream_id, event_index, payload, payload_is_null, payload_size, created_at
FROM mmmcp_event_log;

DROP TABLE mmmcp_event_log;
DROP TABLE mmmcp_event_streams;

CREATE INDEX IF NOT EXISTS mmcp_event_streams_expiry_idx
    ON mmcp_event_streams (expires_at);

CREATE INDEX IF NOT EXISTS mmcp_event_log_created_idx
    ON mmcp_event_log (created_at, session_id, stream_id, event_index);
