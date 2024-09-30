# EO DataHub Workspace Services
An API gateway to the EO DataHub to manage workspace related API requests

## Getting Started
### Requisites
- Go 1.16 or higher

### Installation
Clone the repository:
```
git clone git@github.com:EO-DataHub/eodhp-workspace-services.git
cd eodhp-workspace-services
```

## Usage

## Running the Server Locally

### Configuration File
- The repository connects to the workspaces database. In the cluster it will get it's connection string from env vars already setup.
- You can connect to the database when working on the VM via SSH tunnelling, connecting to an AWS ec2 instance which acts as the proxy gateway.
- To do this, you must get the appropriate private key file - `.pem`. Speak to either Jonny Langstone or Steven Gillies who will provide you with it.

- The config file looks like so:
```
database:
  driver: pgx
  source: postgres://{{.SQL_USER}}:{{.SQL_PASSWORD}}@{{.SQL_HOST}}:{{.SQL_PORT}}/{{.SQL_DATABASE}}?search_path={{.SQL_SCHEMA}}

databaseProxy:
  sshUser: ec2-user
  sshHost: <<EC2 INSTANCE HOST>>
  sshPort: 22
  remoteHost: <<AURORA HOST>>
  reportPort: 5432
  localPort: 8443
  privateKeyPath: /path/to/file/eodhp-db-proxy.pem

```


```go run main.go runserver --config /path/to/config.yaml```


## Deployment
- Push to the ECR:
    - ```make VERSION={VERSION} dockerpush```

### Run Locally
```docker build -t workspace-services```
```docker run -p 8080:8080 -e AWS_ACCESS_KEY=.. -e AWS_SECRET_ACCESS_KEY=.. workspace-services ```