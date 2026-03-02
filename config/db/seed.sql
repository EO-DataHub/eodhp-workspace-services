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
) VALUES
(
    '22222222-2222-2222-2222-222222222221',
    'eodh-demos',
    '11111111-1111-1111-1111-111111111111',
    'Available',
    CURRENT_TIMESTAMP,
    NULL,
    NULL
),
(
    '22222222-2222-2222-2222-222222222222',
    'samples-airbus-optical',
    '11111111-1111-1111-1111-111111111111',
    'Available',
    CURRENT_TIMESTAMP,
    NULL,
    NULL
),
(
    '22222222-2222-2222-2222-222222222223',
    'samples-airbus-sar',
    '11111111-1111-1111-1111-111111111111',
    'Available',
    CURRENT_TIMESTAMP,
    NULL,
    NULL
),
(
    '22222222-2222-2222-2222-222222222224',
    'samples-planet',
    '11111111-1111-1111-1111-111111111111',
    'Available',
    CURRENT_TIMESTAMP,
    NULL,
    NULL
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO workspace_stores (
    id,
    workspace_id,
    store_type,
    name
) VALUES
(
    '33333333-3333-3333-3333-333333333331',
    '22222222-2222-2222-2222-222222222221',
    'object',
    'eodh-demos'
),
(
    '44444444-4444-4444-4444-444444444441',
    '22222222-2222-2222-2222-222222222221',
    'block',
    'eodh-demos'
),
(
    '33333333-3333-3333-3333-333333333332',
    '22222222-2222-2222-2222-222222222222',
    'object',
    'samples-airbus-optical'
),
(
    '44444444-4444-4444-4444-444444444442',
    '22222222-2222-2222-2222-222222222222',
    'block',
    'samples-airbus-optical'
),
(
    '33333333-3333-3333-3333-333333333333',
    '22222222-2222-2222-2222-222222222223',
    'object',
    'samples-airbus-sar'
),
(
    '44444444-4444-4444-4444-444444444443',
    '22222222-2222-2222-2222-222222222223',
    'block',
    'samples-airbus-sar'
),
(
    '33333333-3333-3333-3333-333333333334',
    '22222222-2222-2222-2222-222222222224',
    'object',
    'samples-planet'
),
(
    '44444444-4444-4444-4444-444444444444',
    '22222222-2222-2222-2222-222222222224',
    'block',
    'samples-planet'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO object_stores (
    store_id,
    path,
    env_var,
    access_point_arn
) VALUES
(
    '33333333-3333-3333-3333-333333333331',
    'eodh-demos',
    'EODH_DEMOS',
    'arn:aws:s3:::eodh-demos'
),
(
    '33333333-3333-3333-3333-333333333332',
    'samples-airbus-optical',
    'SAMPLES_AIRBUS_OPTICAL',
    'arn:aws:s3:::samples-airbus-optical'
),
(
    '33333333-3333-3333-3333-333333333333',
    'samples-airbus-sar',
    'SAMPLES_AIRBUS_SAR',
    'arn:aws:s3:::samples-airbus-sar'
),
(
    '33333333-3333-3333-3333-333333333334',
    'samples-planet',
    'SAMPLES_PLANET',
    'arn:aws:s3:::samples-planet'
)
ON CONFLICT (store_id) DO NOTHING;

INSERT INTO block_stores (
    store_id,
    access_point_id,
    mount_point
) VALUES
(
    '44444444-4444-4444-4444-444444444441',
    'local-ap',
    '/eodh-demos'
),
(
    '44444444-4444-4444-4444-444444444442',
    'local-ap',
    '/samples-airbus-optical'
),
(
    '44444444-4444-4444-4444-444444444443',
    'local-ap',
    '/samples-airbus-sar'
),
(
    '44444444-4444-4444-4444-444444444444',
    'local-ap',
    '/samples-planet'
)
ON CONFLICT (store_id) DO NOTHING;
