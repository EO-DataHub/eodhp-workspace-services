-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
ALTER TABLE accounts ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'Pending';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
ALTER TABLE accounts DROP COLUMN status;
-- +goose StatementEnd
