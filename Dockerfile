# diffbox - Self-hosted AI video/image generation
# Single container for RunPod/Vast.ai deployment

FROM nvidia/cuda:12.1.1-runtime-ubuntu22.04

# Prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies including Redis (Valkey-compatible)
RUN apt-get update && apt-get install -y \
    aria2 \
    curl \
    git \
    ffmpeg \
    redis-server \
    && rm -rf /var/lib/apt/lists/*

# Create symlink for valkey-server (Redis is API-compatible)
RUN ln -s /usr/bin/redis-server /usr/local/bin/valkey-server

# Install uv (Python package manager)
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# Create app directory
WORKDIR /app

# Copy Go binary (built externally)
COPY diffbox /usr/local/bin/diffbox

# Copy Python worker
COPY python /app/python

# Install Python dependencies
WORKDIR /app/python
RUN uv sync --frozen

# Copy frontend static files
COPY web/dist /app/static

# Create directories for runtime data
RUN mkdir -p /data /models /outputs

# Set environment variables
ENV DIFFBOX_PORT=8080
ENV DIFFBOX_DATA_DIR=/data
ENV DIFFBOX_MODELS_DIR=/models
ENV DIFFBOX_OUTPUTS_DIR=/outputs
ENV DIFFBOX_STATIC_DIR=/app/static
ENV DIFFBOX_PYTHON_PATH=/app/python

# Declare persistent volumes (for RunPod/Vast.ai)
VOLUME ["/data", "/models", "/outputs"]

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -f http://localhost:8080/api/health || exit 1

# Run diffbox (which spawns Redis/Valkey, aria2, and Python workers)
WORKDIR /app
ENTRYPOINT ["/usr/local/bin/diffbox"]
