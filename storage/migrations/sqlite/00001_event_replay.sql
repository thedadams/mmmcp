-- +goose Up
CREATE TABLE mmcp_event_streams (
    session_id TEXT NOT NULL,
    stream_id TEXT NOT NULL,
    first_retained_index INTEGER NOT NULL DEFAULT 0,
    next_index INTEGER NOT NULL DEFAULT 0,
    expires_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (session_id, stream_id)
);

CREATE TABLE mmcp_event_log (
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

CREATE INDEX mmcp_event_streams_expiry_idx
    ON mmcp_event_streams (expires_at);

CREATE INDEX mmcp_event_log_created_idx
    ON mmcp_event_log (created_at, session_id, stream_id, event_index);

-- +goose Down
DROP TABLE IF EXISTS mmcp_event_log;
DROP TABLE IF EXISTS mmcp_event_streams;
