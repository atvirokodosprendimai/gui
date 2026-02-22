-- +goose Up
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_sessions_expires_at;
