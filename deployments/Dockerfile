FROM golang:1.25-bookworm AS builder

WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends build-essential && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go test ./internal/wecom/sdk
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -o /server ./cmd/server

FROM debian:bookworm-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata zlib1g && rm -rf /var/lib/apt/lists/*
COPY --from=builder /server /app/server
COPY config ./config
EXPOSE 8080
ENTRYPOINT ["/app/server"]
