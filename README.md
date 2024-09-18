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

## Running the Server

```go run main.go runserver --help ```


## Deployment
- Push to the ECR:
    - ```make VERSION={VERSION} dockerpush```

### Run Locally
```docker build -t workspace-services```
```docker run -p 8080:8080 -e AWS_ACCESS_KEY=.. -e AWS_SECRET_ACCESS_KEY=.. workspace-services ```