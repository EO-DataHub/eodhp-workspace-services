-- +goose Up
-- +goose StatementBegin
-- Pricing category used by accounting-service. NULL means no category has been set and the default rate applies.
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS category TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workspaces DROP COLUMN IF EXISTS category;
-- +goose StatementEnd
