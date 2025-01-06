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
database:
  driver: pgx
  source: postgres://{{.SQL_USER}}:{{.SQL_PASSWORD}}@{{.SQL_HOST}}:{{.SQL_PORT}}/{{.SQL_DATABASE}}?search_path={{.SQL_SCHEMA}}
pulsar:
  url: pulsar://<<REPLACE>>
  topicProducer: persistent://public/default/workspace-settings
  topicConsumer: persistent://public/default/workspace-status
  subscription: workspace-status-sub
```
The repository connects to the workspaces database. In the cluster it will get it's connection string from env vars already setup. This config file is defined in the `eodhp-argocd-deployment` `app/workspace-services/base/config.yaml`


## Run with AWS DB Locally
If you want to connect to the AWS instance, you should set up an SSH tunnel outside the go app in a separate terminal:

`ssh -i <PATH-TO-PRIVATE-KEY> -L 8443:<REMOTE-HOST>:5432 <SSH-USER>@<SSH-HOST> -N`

These details can be given upon request.



## Run with Pulsar Locally
Make sure pulsar is installed. If you run `./pulsar standalone` and amend the config file to your localhost, then the app will attach to it.

## Run the API Server:
To start the server:

```go run main.go serve --config /path/to/config.yaml```



### Consume Workspace Events
The `consume` command runs a Pulsar consumer that listens to events on the `workspace-status` topic and processes workspace updates in the database.

To start the server:

```go run main.go consume --config /path/to/config.yaml```

## Deployment
- Push to the ECR:
    - ```make VERSION={VERSION} dockerpush```

### Run Locally
```docker build -t workspace-services```
```docker run -p 8080:8080 -e AWS_ACCESS_KEY=.. -e AWS_SECRET_ACCESS_KEY=.. workspace-services ```