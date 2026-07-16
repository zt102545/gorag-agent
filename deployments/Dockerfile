FROM quay.io/pypa/manylinux2014_x86_64:latest AS builder

ARG GO_VERSION=1.25.1
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /server ./cmd/server

FROM debian:bookworm-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata zlib1g && rm -rf /var/lib/apt/lists/*
COPY --from=builder /server /app/server
COPY config ./config
EXPOSE 8080
ENTRYPOINT ["/app/server"]
