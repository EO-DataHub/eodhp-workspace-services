-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE IF NOT EXISTS account_approvals (
	id UUID PRIMARY KEY,
	account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	approval_token VARCHAR(255) UNIQUE NOT NULL,
	token_expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
DROP TABLE IF EXISTS account_approvals;
-- +goose StatementEnd
