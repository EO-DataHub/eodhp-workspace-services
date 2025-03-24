-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
CREATE TABLE IF NOT EXISTS accounts (
				id UUID PRIMARY KEY,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				name VARCHAR(255) NOT NULL,
				account_owner TEXT NOT NULL,
				billing_address TEXT NOT NULL,
				organization_name TEXT NULL,
				account_opening_reason TEXT NULL
);
CREATE TABLE IF NOT EXISTS workspaces (
				id UUID PRIMARY KEY,
				name VARCHAR(255) UNIQUE NOT NULL,
				account UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
				member_group TEXT NOT NULL,
				role_name TEXT NULL,
				role_arn TEXT NULL,
				status TEXT NOT NULL,
				last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS workspace_stores (
				id UUID PRIMARY KEY,
				workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
				store_type VARCHAR(50) NOT NULL,
				name VARCHAR(255) NOT NULL
);
CREATE TABLE IF NOT EXISTS object_stores (
				store_id UUID PRIMARY KEY REFERENCES workspace_stores(id) ON DELETE CASCADE,
				path VARCHAR(255) NOT NULL,
				env_var VARCHAR(255) NOT NULL,
				access_point_arn VARCHAR(255) NOT NULL
);
CREATE TABLE IF NOT EXISTS block_stores (
				store_id UUID PRIMARY KEY REFERENCES workspace_stores(id) ON DELETE CASCADE,
				access_point_id VARCHAR(255) NOT NULL,
				mount_point VARCHAR(255) NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
