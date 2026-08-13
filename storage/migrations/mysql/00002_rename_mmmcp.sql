-- +goose Up
RENAME TABLE
    mmcp_event_streams TO mmmcp_event_streams,
    mmcp_event_log TO mmmcp_event_log;

ALTER TABLE mmmcp_event_streams
    RENAME INDEX mmcp_event_streams_expiry_idx TO mmmcp_event_streams_expiry_idx;
ALTER TABLE mmmcp_event_log
    RENAME INDEX mmcp_event_log_created_idx TO mmmcp_event_log_created_idx;
ALTER TABLE mmmcp_event_log
    DROP FOREIGN KEY mmcp_event_log_stream_fk,
    ADD CONSTRAINT mmmcp_event_log_stream_fk
        FOREIGN KEY (session_id, stream_id)
        REFERENCES mmmcp_event_streams (session_id, stream_id)
        ON DELETE CASCADE;

-- +goose Down
ALTER TABLE mmmcp_event_log
    DROP FOREIGN KEY mmmcp_event_log_stream_fk,
    ADD CONSTRAINT mmcp_event_log_stream_fk
        FOREIGN KEY (session_id, stream_id)
        REFERENCES mmmcp_event_streams (session_id, stream_id)
        ON DELETE CASCADE;
ALTER TABLE mmmcp_event_streams
    RENAME INDEX mmmcp_event_streams_expiry_idx TO mmcp_event_streams_expiry_idx;
ALTER TABLE mmmcp_event_log
    RENAME INDEX mmmcp_event_log_created_idx TO mmcp_event_log_created_idx;

RENAME TABLE
    mmmcp_event_log TO mmcp_event_log,
    mmmcp_event_streams TO mmcp_event_streams;
