.PHONY: build docker-build docker-up docker-down docker-logs clean test

# Binary name
BINARY := cortex
BUILD_DIR := bin

# Go build flags
GO_FLAGS := CGO_ENABLED=0
LDFLAGS := -trimpath -ldflags "-s -w"

# Build the binary locally
build:
	@mkdir -p $(BUILD_DIR)
	$(GO_FLAGS) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/mcpserver

# Build Docker images
docker-build:
	docker-compose build

# Start Docker services
docker-up:
	docker-compose up -d

# Stop Docker services
docker-down:
	docker-compose down

# View Docker logs
docker-logs:
	docker-compose logs -f

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	docker-compose down -v --rmi local 2>/dev/null || true

# Development: rebuild and restart
dev: docker-build docker-up docker-logs
