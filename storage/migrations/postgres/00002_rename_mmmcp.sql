-- +goose Up
ALTER TABLE mmcp_event_streams RENAME TO mmmcp_event_streams;
ALTER TABLE mmcp_event_log RENAME TO mmmcp_event_log;
ALTER INDEX mmcp_event_streams_expiry_idx RENAME TO mmmcp_event_streams_expiry_idx;
ALTER INDEX mmcp_event_log_created_idx RENAME TO mmmcp_event_log_created_idx;
ALTER TABLE mmmcp_event_log
    RENAME CONSTRAINT mmcp_event_log_session_id_stream_id_fkey
    TO mmmcp_event_log_session_id_stream_id_fkey;

-- +goose Down
ALTER TABLE mmmcp_event_log
    RENAME CONSTRAINT mmmcp_event_log_session_id_stream_id_fkey
    TO mmcp_event_log_session_id_stream_id_fkey;
ALTER INDEX mmmcp_event_streams_expiry_idx RENAME TO mmcp_event_streams_expiry_idx;
ALTER INDEX mmmcp_event_log_created_idx RENAME TO mmcp_event_log_created_idx;
ALTER TABLE mmmcp_event_log RENAME TO mmcp_event_log;
ALTER TABLE mmmcp_event_streams RENAME TO mmcp_event_streams;
