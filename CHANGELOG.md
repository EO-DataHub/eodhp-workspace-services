# Workspace Services

## v0.6.9 (20-02-2025)

- Getting group membership from keycloak API instead of claims
- Added OpenAPI doc generation and endpoint to view them (s3 only)
- Get S3 credentials from user scoped token

## v0.6.6 (14-02-2025)

- Added more logging
- Added new columns to billing accounts

## v0.6.5 (12-02-2025)

- Revert to using workspace name as object/block store names

## v0.6.4 (06-02-2025)

- Using `username` instead of `user-id` to add/remove users to/from a workspace

## v0.6.3 (04-02-2025)

- App is more RESTful
- Removed implementation details in API response not needed (e.g. `fsID`)
- Added `host`, `bucket`, `prefix` to the object store API response
- Added `mount_point` to the block store API response

## v0.6.1 (27-01-2025)

- Account owner of a workspace autoamtically added to the KC group
- Created system defined default object/block stores upon workspace creation

## v0.6.0 (14-01-2025)

- Added Workspace management endpoints
- More RESTful system architecture

