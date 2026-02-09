INSERT INTO accounts (
    id,
    name,
    account_owner,
    billing_address,
    organization_name,
    account_opening_reason,
    status
) VALUES (
    '11111111-1111-1111-1111-111111111111',
    'dev-account',
    'dev-user',
    'local',
    'local',
    'dev',
    'Approved'
) ON CONFLICT (id) DO NOTHING;

INSERT INTO workspaces (
    id,
    name,
    account,
    status,
    last_updated,
    role_name,
    role_arn
) VALUES (
    '22222222-2222-2222-2222-222222222222',
    'test-workspace',
    '11111111-1111-1111-1111-111111111111',
    'Available',
    CURRENT_TIMESTAMP,
    NULL,
    NULL
) ON CONFLICT (id) DO NOTHING;

INSERT INTO workspace_stores (
    id,
    workspace_id,
    store_type,
    name
) VALUES
(
    '33333333-3333-3333-3333-333333333333',
    '22222222-2222-2222-2222-222222222222',
    'object',
    'test-workspace'
),
(
    '44444444-4444-4444-4444-444444444444',
    '22222222-2222-2222-2222-222222222222',
    'block',
    'test-workspace'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO object_stores (
    store_id,
    path,
    env_var,
    access_point_arn
) VALUES (
    '33333333-3333-3333-3333-333333333333',
    'test-workspace',
    'TEST',
    'arn:aws:s3:::test-workspace'
) ON CONFLICT (store_id) DO NOTHING;

INSERT INTO block_stores (
    store_id,
    access_point_id,
    mount_point
) VALUES (
    '44444444-4444-4444-4444-444444444444',
    'local-ap',
    '/data/block/test-workspace'
) ON CONFLICT (store_id) DO NOTHING;
