.PHONY: dockerbuild dockerpush test lint format setup clean

# Variables
VERSION ?= latest
IMAGENAME = eodhp-workspace-services
DOCKERREPO ?= public.ecr.aws/eodh

# Docker build with BuildKit and build arguments for OS and architecture
dockerbuild:
	DOCKER_BUILDKIT=1 docker build --build-arg TARGETOS=linux --build-arg TARGETARCH=amd64 -t ${IMAGENAME}:${VERSION} .

# Docker push
dockerpush: dockerbuild
	docker tag ${IMAGENAME}:${VERSION} ${DOCKERREPO}/${IMAGENAME}:${VERSION}
	docker push ${DOCKERREPO}/${IMAGENAME}:${VERSION}

# Run Go tests
test:
	go test ./...

# Lint using go vet and golangci-lint
lint: 
	go vet ./...
	golangci-lint run

# Format code using gofmt
format:
	go fmt ./...

# Install dependencies
deps:
	go mod tidy
	go mod vendor

# Setup development environment (install Go tools, dependencies, etc.)
setup: deps
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Clean up build artifacts
clean:
	go clean
	rm -rf bin/*

# Build the Go binary
build:
	go build -o bin/${IMAGENAME} main.go

# Test and Docker build
testdocker: test dockerbuild

# Default target to build, test, and push Docker image
all: test dockerbuild dockerpush