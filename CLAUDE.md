# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

diffbox is a self-hosted web app for AI video/image generation, optimized for ephemeral cloud deployments (RunPod, Vast.ai). It uses **ComfyUI** as the inference backend via HTTP/WebSocket API. Supports three workflows: Wan 2.2 I2V (image-to-video), Qwen Image Edit (instruction-based editing), and Dolphin-Mistral chat (NSFW prompt generation).

## Commands

```bash
# Initialize development environment (first-time setup)
make init

# Development with hot reload (requires air, auto-installed)
make dev

# Build Go binary
make build

# Build frontend only
make frontend

# Run tests (Go + Python)
make test

# Format all code (Go, Python, TypeScript)
make fmt

# Lint all code
make lint

# Build Docker image (development)
make docker

# Run Docker locally with GPU
make docker-run

# RunPod deployment (single container with ComfyUI)
make docker-runpod              # Build optimized RunPod image
make test-runpod                # Test locally (requires GPU)
make push-runpod REGISTRY=user  # Push to Docker Hub
```

**Frontend-only development:**
```bash
cd web && npm run dev     # Vite dev server (proxies to :8080)
cd web && npm run build   # TypeScript check + production build
cd web && npm run lint    # ESLint
cd web && npm run format  # Prettier
```

**Python worker testing:**
```bash
make python-deps          # Install dependencies with uv
make python-worker        # Run worker directly
cd python && uv run pytest  # Run Python tests only
```

## Architecture

```
React UI → REST API (Go/Chi) → Redis Streams (Valkey) → Python Workers → ComfyUI API
                ↓                                            ↓                ↓
          WebSocket Hub ←──────── Progress Updates ←────────┴────────────────┘
```

**Process model (RunPod):** Single container runs:
- **supervisord** (PID 1) managing:
  - ComfyUI server (port 8188) - GPU inference backend
  - diffbox server (port 8080) - Go API server + spawned Python workers
  - Valkey (Redis) + aria2 (model downloads)

### Key Directories

- `cmd/server/` - Go entrypoint, spawns Valkey/aria2/workers
- `internal/api/` - HTTP handlers (Chi router), WebSocket hub
- `internal/worker/` - Python worker lifecycle management
- `internal/queue/` - Redis Streams job queue abstraction
- `internal/db/` - SQLite persistence
- `python/worker/` - Inference workers (i2v.py, qwen.py, chat.py)
- `python/worker/comfyui_client.py` - ComfyUI HTTP/WebSocket client (TODO: implement)
- `web/src/pages/` - WorkflowPage, ModelsPage, SettingsPage
- `web/src/hooks/` - useWebSocket for real-time updates
- `supervisord.conf` - Process manager config for RunPod
- `Dockerfile.runpod` - Single-container RunPod build

### Tech Stack

| Layer | Stack |
|-------|-------|
| Frontend | React 19, TypeScript, TanStack Query, Zustand, Tailwind CSS |
| Backend | Go 1.23, Chi router, SQLite, Bleve search |
| Queue | Valkey (Redis fork) + Redis Streams |
| Python | uv, PyTorch, vLLM (chat), aiohttp/websockets (ComfyUI client) |
| Inference | ComfyUI (HTTP/WebSocket API) |
| Downloads | aria2 JSON-RPC |
| Deployment | supervisord (process manager) |

## API Endpoints

```
POST /api/workflows/{i2v,svi,qwen}  - Submit job
GET  /api/jobs                      - List jobs
GET  /api/jobs/{id}                 - Get job
DELETE /api/jobs/{id}               - Cancel job
GET  /api/models                    - Search models
POST /api/models/{source}/{id}/download
GET  /api/config                    - Export config
POST /api/config                    - Import config
GET  /ws                            - WebSocket (real-time progress)
```

## Communication Flow

1. **Go ↔ Python**: stdin/stdout JSON protocol (job dispatch, progress updates)
2. **Python ↔ ComfyUI**: HTTP (workflow submission) + WebSocket (progress tracking)
3. **Backend ↔ Frontend**: REST API (job control) + WebSocket (real-time updates)

## Environment Variables

```bash
DIFFBOX_PORT=8080
DIFFBOX_DATA_DIR=/data
DIFFBOX_MODELS_DIR=/models
DIFFBOX_OUTPUTS_DIR=/outputs
DIFFBOX_VALKEY_ADDR=localhost:6379
```

## Documentation

- `docs/architecture.md` - Detailed technical specification
- `docs/handover.md` - Developer context and design decisions
- `docs/runpod-deployment.md` - RunPod deployment guide
- `docs/comfyui-integration.md` - ComfyUI integration details (TODO: create)
