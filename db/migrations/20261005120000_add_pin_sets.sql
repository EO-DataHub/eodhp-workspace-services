-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS pin_sets (
	id UUID PRIMARY KEY,
	workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
	name VARCHAR(100) NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	visibility TEXT NOT NULL DEFAULT 'workspace' CHECK (visibility IN ('workspace', 'private')),
	created_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE (workspace_id, name)
);

CREATE TABLE IF NOT EXISTS pin_set_items (
	id UUID PRIMARY KEY,
	set_id UUID NOT NULL REFERENCES pin_sets(id) ON DELETE CASCADE,
	collection_id TEXT NOT NULL,
	item_id TEXT NOT NULL,
	self_href TEXT NOT NULL,
	display JSONB NULL,
	position INTEGER NOT NULL,
	added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE (set_id, self_href)
);

CREATE INDEX IF NOT EXISTS pin_set_items_set_position_idx ON pin_set_items (set_id, position);
-- Not used by any query yet. It is for finding the sets that contain a given STAC item.
CREATE INDEX IF NOT EXISTS pin_set_items_item_idx ON pin_set_items (collection_id, item_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pin_set_items;
DROP TABLE IF EXISTS pin_sets;
-- +goose StatementEnd
