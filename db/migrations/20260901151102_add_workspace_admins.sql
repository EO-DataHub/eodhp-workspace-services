-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS workspace_admins (
	workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	username TEXT NOT NULL,
	added_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (workspace_id, username)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workspace_admins;
-- +goose StatementEnd
