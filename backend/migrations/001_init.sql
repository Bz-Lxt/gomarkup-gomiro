-- +migrate Up
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS boards (
    id            TEXT PRIMARY KEY,
    title         TEXT NOT NULL,
    pass_hash     TEXT NOT NULL DEFAULT '',
    thumbnail     TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_boards_updated ON boards (updated_at DESC);

CREATE TABLE IF NOT EXISTS board_snapshots (
    board_id      TEXT PRIMARY KEY REFERENCES boards(id) ON DELETE CASCADE,
    server_seq    BIGINT NOT NULL,
    payload       JSONB NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS op_logs (
    board_id      TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    server_seq    BIGINT NOT NULL,
    author_id     TEXT NOT NULL,
    kind          TEXT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (board_id, server_seq)
);

CREATE INDEX IF NOT EXISTS idx_oplog_board_seq ON op_logs (board_id, server_seq);

CREATE TABLE IF NOT EXISTS uploads (
    hash          TEXT PRIMARY KEY,
    mime          TEXT NOT NULL,
    bytes         INTEGER NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL
);

-- +migrate Down
DROP TABLE IF EXISTS uploads;
DROP TABLE IF EXISTS op_logs;
DROP TABLE IF EXISTS board_snapshots;
DROP TABLE IF EXISTS boards;
DROP TABLE IF EXISTS schema_migrations;
