.PHONY: all build run dev clean docker test

# Variables
BINARY_NAME=diffbox
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*')
DOCKER_IMAGE=diffbox

# Default target
all: build

# Build Go binary
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) ./cmd/server

# Run locally (development)
run: build
	./$(BINARY_NAME)

# Development mode with hot reload (requires air)
dev:
	@which air > /dev/null || go install github.com/air-verse/air@latest
	air

# Build frontend
frontend:
	@echo "Building frontend..."
	cd web && npm install && npm run build

# Build Docker image
docker: frontend
	@echo "Building Docker image..."
	CGO_ENABLED=1 GOOS=linux go build -o $(BINARY_NAME) ./cmd/server
	docker build -t $(DOCKER_IMAGE) .

# Run Docker container locally
docker-run:
	docker run --gpus all -p 8080:8080 \
		-v $(PWD)/data:/data \
		-v $(PWD)/models:/models \
		-v $(PWD)/outputs:/outputs \
		$(DOCKER_IMAGE)

# Install Python dependencies
python-deps:
	cd python && uv sync --extra dev

# Run Python worker directly (for testing)
python-worker:
	cd python && uv run python -m worker

# Run tests
test:
	go test -v ./...
	cd python && uv run pytest

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf web/dist
	rm -rf data/*.db
	rm -rf data/search.bleve

# Format code
fmt:
	go fmt ./...
	cd python && uv run black .
	cd web && npm run format

# Lint code
lint:
	go vet ./...
	cd python && uv run ruff check .
	cd web && npm run lint

# Generate Go dependencies
deps:
	go mod download
	go mod tidy

# Initialize development environment
init: deps python-deps
	cd web && npm install
	@echo "Development environment initialized!"
	@echo "Run 'make dev' to start the server"
