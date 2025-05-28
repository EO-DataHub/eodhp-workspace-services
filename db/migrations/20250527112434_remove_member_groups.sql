-- +goose Up
-- +goose StatementBegin
ALTER TABLE workspaces DROP COLUMN IF EXISTS member_group;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workspaces ADD COLUMN member_group TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd
