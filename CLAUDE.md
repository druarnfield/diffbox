# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

diffbox is a self-hosted web app for AI video/image generation, optimized for ephemeral cloud deployments (RunPod, Vast.ai). It supports three workflows: Wan 2.2 I2V (image-to-video), Wan 2.2 SVI 2.0 Pro (infinite video), and Qwen Image Edit (instruction-based editing with inpainting).

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

# Build Docker image
make docker

# Run Docker locally with GPU
make docker-run
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
React UI → REST API (Go/Chi) → Redis Streams (Valkey) → Python Workers
                ↓                                            ↓
          WebSocket Hub ←──────── Progress Updates ←─────────┘
```

**Process model (Docker):** Single container runs Go server (PID 1), Valkey, aria2, and spawned Python workers.

### Key Directories

- `cmd/server/` - Go entrypoint, spawns Valkey/aria2/workers
- `internal/api/` - HTTP handlers (Chi router), WebSocket hub
- `internal/worker/` - Python worker lifecycle management
- `internal/queue/` - Redis Streams job queue abstraction
- `internal/db/` - SQLite persistence
- `python/worker/` - Inference workers (i2v.py, svi.py, qwen.py)
- `python/diffsynth/` - Vendored inference library
- `web/src/pages/` - WorkflowPage, ModelsPage, SettingsPage
- `web/src/hooks/` - useWebSocket for real-time updates

### Tech Stack

| Layer | Stack |
|-------|-------|
| Frontend | React 19, TypeScript, TanStack Query, Zustand, Tailwind CSS |
| Backend | Go 1.23, Chi router, SQLite, Bleve search |
| Queue | Valkey (Redis fork) + Redis Streams |
| Python | uv, PyTorch, diffsynth (vendored) |
| Downloads | aria2 JSON-RPC |

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

## Go-Python Communication

Workers communicate via stdin/stdout JSON protocol. The Go server spawns workers and sends job payloads; workers emit progress updates that flow back through WebSocket to the frontend.

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
