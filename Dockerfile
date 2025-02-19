FROM golang:1.23 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
RUN go install github.com/swaggo/swag/cmd/swag@latest
COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
COPY api/ api/
COPY cmd/ cmd/
COPY db/ db/
COPY docs/ docs/
COPY internal/ internal/
COPY models/ models/

# Generate Swagger docs
RUN swag init -g cmd/serve.go

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a \
    -o app main.go

#FROM gcr.io/distroless/static:nonroot
FROM alpine:latest


COPY --from=builder /workspace/app /usr/local/bin/app
USER 65532:65532

EXPOSE 8080
ENTRYPOINT ["app"]
CMD ["serve", "--host=0.0.0.0", "--port=8080"]