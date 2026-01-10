# Stage 1: Build
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o cortex ./cmd/mcpserver

# Stage 2: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/cortex /usr/local/bin/cortex

# For MCP stdio mode via docker-compose exec, we need a keep-alive process
# The actual cortex binary is invoked via: docker-compose exec -T cortex /usr/local/bin/cortex
# For standalone mode (docker-compose up), cortex runs as the main process
ENTRYPOINT ["/usr/local/bin/cortex"]
