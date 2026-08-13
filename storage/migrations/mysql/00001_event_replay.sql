-- +goose Up
CREATE TABLE mmcp_event_streams (
    session_id VARCHAR(255) NOT NULL,
    stream_id VARCHAR(255) NOT NULL,
    first_retained_index BIGINT NOT NULL DEFAULT 0,
    next_index BIGINT NOT NULL DEFAULT 0,
    expires_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (session_id, stream_id),
    INDEX mmcp_event_streams_expiry_idx (expires_at)
) ENGINE=InnoDB;

CREATE TABLE mmcp_event_log (
    session_id VARCHAR(255) NOT NULL,
    stream_id VARCHAR(255) NOT NULL,
    event_index BIGINT NOT NULL,
    payload LONGBLOB NULL,
    payload_is_null TINYINT NOT NULL DEFAULT 0,
    payload_size BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    PRIMARY KEY (session_id, stream_id, event_index),
    INDEX mmcp_event_log_created_idx (created_at, session_id, stream_id, event_index),
    CONSTRAINT mmcp_event_log_stream_fk
        FOREIGN KEY (session_id, stream_id)
        REFERENCES mmcp_event_streams (session_id, stream_id)
        ON DELETE CASCADE
) ENGINE=InnoDB;

-- +goose Down
DROP TABLE IF EXISTS mmcp_event_log;
DROP TABLE IF EXISTS mmcp_event_streams;
