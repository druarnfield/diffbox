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
	docker build -t $(DOCKER_IMAGE):latest -t $(DOCKER_IMAGE):dev .

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
	cd python && uv sync --extra dev && uv run pytest

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

# RunPod deployment targets
.PHONY: docker-runpod test-runpod push-runpod

# Build RunPod-optimized single container
docker-runpod:
	@echo "Building RunPod optimized image (this will take 10-15 minutes)..."
	docker build -f Dockerfile.runpod -t $(DOCKER_IMAGE)-runpod:latest .
	@echo ""
	@echo "✓ RunPod image built successfully!"
	@echo "  Image: $(DOCKER_IMAGE)-runpod:latest"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Test locally: make test-runpod"
	@echo "  2. Push to registry: make push-runpod REGISTRY=your-username"
	@echo "  3. Deploy to RunPod using: your-username/$(DOCKER_IMAGE)-runpod:latest"

# Test RunPod container locally (requires GPU)
test-runpod:
	@echo "Testing RunPod container locally..."
	@echo "This requires:"
	@echo "  - NVIDIA GPU"
	@echo "  - nvidia-docker installed"
	@echo ""
	docker run --rm --gpus all \
		-p 8080:8080 \
		-p 8188:8188 \
		-v $(PWD)/models:/models \
		-v $(PWD)/outputs:/outputs \
		-v $(PWD)/data:/data \
		$(DOCKER_IMAGE)-runpod:latest

# Push to container registry
# Usage: make push-runpod REGISTRY=your-dockerhub-username
push-runpod:
ifndef REGISTRY
	@echo "Error: REGISTRY not set"
	@echo "Usage: make push-runpod REGISTRY=your-dockerhub-username"
	@exit 1
endif
	@echo "Tagging and pushing to $(REGISTRY)/$(DOCKER_IMAGE)-runpod:latest..."
	docker tag $(DOCKER_IMAGE)-runpod:latest $(REGISTRY)/$(DOCKER_IMAGE)-runpod:latest
	docker push $(REGISTRY)/$(DOCKER_IMAGE)-runpod:latest
	@echo ""
	@echo "✓ Image pushed successfully!"
	@echo "  Registry: $(REGISTRY)/$(DOCKER_IMAGE)-runpod:latest"
	@echo ""
	@echo "Deploy to RunPod:"
	@echo "  1. Go to https://runpod.io/console/pods"
	@echo "  2. Click 'Deploy'"
	@echo "  3. Select GPU (RTX 4090 or A100 recommended)"
	@echo "  4. Container Image: $(REGISTRY)/$(DOCKER_IMAGE)-runpod:latest"
	@echo "  5. Expose HTTP Port: 8080"
	@echo "  6. Volume: /workspace/models -> /models"
	@echo "  7. Volume: /workspace/outputs -> /outputs"
	@echo "  8. Volume: /workspace/data -> /data"

# RunPod Serverless targets
.PHONY: docker-serverless push-serverless

# Build RunPod Serverless image (GPU inference only)
docker-serverless:
	@echo "Building RunPod Serverless image..."
	docker build -f Dockerfile.serverless -t $(DOCKER_IMAGE)-serverless:latest .
	@echo ""
	@echo "✓ Serverless image built successfully!"
	@echo "  Image: $(DOCKER_IMAGE)-serverless:latest"
	@echo ""
	@echo "This image contains:"
	@echo "  - ComfyUI (I2V, Qwen workflows)"
	@echo "  - Dolphin-Mistral chat model"
	@echo "  - RunPod serverless handler"
	@echo ""
	@echo "Next: make push-serverless REGISTRY=your-username"

# Push serverless image to registry
# Usage: make push-serverless REGISTRY=your-dockerhub-username
push-serverless:
ifndef REGISTRY
	@echo "Error: REGISTRY not set"
	@echo "Usage: make push-serverless REGISTRY=your-dockerhub-username"
	@exit 1
endif
	@echo "Tagging and pushing to $(REGISTRY)/$(DOCKER_IMAGE)-serverless:latest..."
	docker tag $(DOCKER_IMAGE)-serverless:latest $(REGISTRY)/$(DOCKER_IMAGE)-serverless:latest
	docker push $(REGISTRY)/$(DOCKER_IMAGE)-serverless:latest
	@echo ""
	@echo "✓ Image pushed successfully!"
	@echo "  Registry: $(REGISTRY)/$(DOCKER_IMAGE)-serverless:latest"
	@echo ""
	@echo "Deploy to RunPod Serverless:"
	@echo "  1. Go to https://runpod.io/console/serverless"
	@echo "  2. Create New Endpoint"
	@echo "  3. Container Image: $(REGISTRY)/$(DOCKER_IMAGE)-serverless:latest"
	@echo "  4. GPU: RTX 4090 or A40 (24GB+ VRAM)"
	@echo "  5. Network Volume: Create 100GB volume for models"
	@echo "  6. Mount path: /runpod-volume"
	@echo "  7. Copy endpoint ID for RUNPOD_ENDPOINT_ID env var"
