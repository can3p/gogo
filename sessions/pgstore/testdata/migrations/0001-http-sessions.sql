-- +migrate Up
CREATE TABLE IF NOT EXISTS http_sessions (
    id BIGSERIAL PRIMARY KEY,
    key BYTEA,
    data BYTEA,
    created_on TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMPTZ,
    expires_on TIMESTAMPTZ);
CREATE INDEX IF NOT EXISTS http_sessions_expiry_idx ON http_sessions (expires_on);
CREATE INDEX IF NOT EXISTS http_sessions_key_idx ON http_sessions (key);

-- +migrate Down
DROP TABLE http_sessions;
