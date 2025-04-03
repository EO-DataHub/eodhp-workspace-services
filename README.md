# EO DataHub Workspace Services
An API gateway to the EO DataHub to manage workspace related API requests and process events for workspace management.

## Getting Started
### Requisites
- Go 1.16 or higher
- Access to a PostgresSQL database
- Apache Pulsar installed locally or access to a Pulsar cluster

### Installation
Clone the repository:
```
git clone git@github.com:EO-DataHub/eodhp-workspace-services.git
cd eodhp-workspace-services
```

## Usage

### Configuration File

The config is in the proposed format:
```
host: {{{ENV}}}.eodatahub.org.uk
basePath: /api
docsPath: /api/docs/workspace-services
accounts:
  serviceAccountEmail: platform@account-verification.{{ENV}}.eodatahub.org.uk
  helpdeskEmail: enquiries@eodatahub.org.uk
database:
  driver: pgx
  source: postgres://{{.SQL_USER}}:{{.SQL_PASSWORD}}@{{.SQL_HOST}}:{{.SQL_PORT}}/{{.SQL_DATABASE}}?search_path={{.SQL_SCHEMA}}
pulsar:
  url: pulsar://<<REPLACE>>
  topicProducer: persistent://public/default/workspace-settings
  topicConsumer: persistent://public/default/workspace-status
  subscription: workspace-status-sub
keycloak:
  url: "https://{{ENV}}.eodatahub.org.uk/keycloak"
  realm: eodhp
  clientId: eodh-workspaces
aws:
  account: {{AWS_ACCOUNT_ID}}
  cluster_prefix: eodhp-{{ENV}}
  region: eu-west-2
  workspace_domain: workspaces.{{ENV}}.eodhp.eco-ke-staging.com
  s3:
    bucket: workspaces-eodhp-{{ENV}}
    host: s3-accesspoint.eu-west-2.amazonaws.com
    roleArn: arn:aws:iam::{{AWS_ACCOUNT_ID}}:role/WorkspaceServices-{{AWS_CLUSTER_NAME}}
providers:
  airbus:
    access_token_url: https://authenticate.foundation.api.oneatlas.airbus.com/auth/realms/IDP/protocol/openid-connect/token
    optical_contracts_url: https://order.api.oneatlas.airbus.com/api/v1/contracts
    sar_contracts_url: https://sar.api.oneatlas.airbus.com/v1/user/whoami
```
The config map is defined in `eodhp-argocd-deployment` `app/workspace-services/base/config.yaml`


## CLI Options
The service has three primary CLI functions:
- API Server (`serve`)
- Workspace Status Updater (`consume`)
- Database Reconciler (`reconcile`)

### API Server
This hosts the API endpoints for billing accounts and workspaces. The API documentation can be viewed at https://staging.eodatahub.org.uk/api/docs/workspace-services/index.html

Run this with:

`go run main.go serve --config {path-to-config.yaml}`


### Workspace Status Updater
This listens for workspace status updates from pulsar topic `persistent://public/default/workspace-status`. It will update the database accordingly.

Run this with:

`go run main.go consume --config {path-to-config.yaml}`


### Database Reconciler
This reconciles workspaces that exist in the database against what exists in the cluster, making sure that the database serves as the source of truth against these resources

Run this with:

`go run main.go reconcile --config {path-to-config.yaml}`

## Local Setup

### Database
To connect to the database, setup an SSH tunnel:

`ssh -i <PATH-TO-PRIVATE-KEY> -L 8443:<REMOTE-HOST>:5432 <SSH-USER>@<SSH-HOST> -N`

A database proxy EC2 instance is used to connect.


### Pulsar
Make sure pulsar is installed. If you run `./pulsar standalone` and amend the config file to your localhost, then the app will attach to it.



## Deployment

```make VERSION={VERSION} dockerpush```
