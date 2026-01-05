# diffbox Developer Handover

> Complete context and decisions for continuing diffbox development.
> Created: January 2025

---

## Table of Contents

1. [Project Vision](#project-vision)
2. [Architectural Decisions](#architectural-decisions)
3. [Tech Stack](#tech-stack)
4. [Workflows & Parameters](#workflows--parameters)
5. [Model Management](#model-management)
6. [Configuration System](#configuration-system)
7. [Project Structure](#project-structure)
8. [What's Been Built](#whats-been-built)
9. [Implementation Phases](#implementation-phases)
10. [Key Files Reference](#key-files-reference)
11. [External Dependencies](#external-dependencies)
12. [Design Decisions & Rationale](#design-decisions--rationale)

---

## Project Vision

**diffbox** is a self-hosted web application for AI video and image generation, specifically designed for **ephemeral cloud deployments** like RunPod and Vast.ai.

### Core Principles

1. **Ephemeral-first**: Users spin up instances, work, then tear them down. Configuration must be exportable/importable for quick restoration.

2. **Opinionated but powerful**: Full parameter control through clean, purpose-built UIs (not a node-based editor like ComfyUI).

3. **Single container deployment**: Everything runs in one Docker container for easy deployment to GPU cloud platforms.

4. **Three focused workflows**:
   - Wan 2.2 I2V (Image-to-Video)
   - Wan 2.2 SVI 2.0 Pro (Infinite/streaming video)
   - Qwen Image Edit (Instruction-based editing with inpainting)

### Target Users

- AI video creators using cloud GPU instances
- Users who want a cleaner alternative to ComfyUI
- Self-hosters who need portable configuration

---

## Architectural Decisions

All major architectural decisions were made through collaborative discussion. Here's what was decided and why:

### Decision 1: Frontend Framework
**Choice**: React + shadcn/ui 2.0

**Considered**: React vs Svelte

**Rationale**:
- Larger ecosystem and component libraries
- shadcn/ui provides polished, accessible components
- Easier to find developers familiar with React

### Decision 2: Backend Language
**Choice**: Go with Chi router

**Considered**: FastAPI (Python), Go, Node/Bun

**Rationale**:
- Originally considered FastAPI but moved away due to "async is so shite" (user's words)
- Go provides simple concurrency model
- Single binary deployment
- Chi is minimal and idiomatic Go

### Decision 3: Python Execution
**Choice**: uv for environment management, Go spawns Python workers

**Rationale**:
- uv is blazing fast for Python environment management
- Clean separation: Go handles HTTP/WebSocket/queue, Python handles inference
- Workers communicate via stdin/stdout JSON protocol

### Decision 4: Job Queue
**Choice**: Valkey + Redis Streams

**Considered**: Celery, ARQ, in-process tasks

**Rationale**:
- Valkey is the open-source Redis fork (post-license change)
- Redis Streams provides persistent, ordered message queue
- Good for progress updates and job management
- Already familiar pattern for ML workloads

### Decision 5: Database
**Choice**: SQLite

**Considered**: PostgreSQL

**Rationale**:
- Zero setup, single file
- Easy backup (just copy the file)
- Sufficient for single-user/small-team self-hosted app
- No extra service to manage

### Decision 6: Full-Text Search
**Choice**: Bleve (Go library)

**Considered**: Blaze (educational), Elasticsearch

**Rationale**:
- Bleve is production-ready, battle-tested
- Embedded in Go binary (no external service)
- Supports BM25, facets, and complex queries
- Used for model browser search

### Decision 7: Model Downloads
**Choice**: aria2 via JSON-RPC

**Rationale**:
- Multi-connection parallel downloads (16x speedup on large files)
- Resume interrupted downloads
- Works with both HuggingFace and Civitai
- Battle-tested in existing SD tools (sd-civitai-browser-plus, batchlinks-webui)

### Decision 8: Deployment
**Choice**: Single Docker container

**Considered**: Docker Compose with separate services

**Rationale**:
- RunPod/Vast.ai work best with single containers
- Go acts as process supervisor (spawns Valkey, aria2, Python workers)
- Simpler deployment story
- No need for dedicated process manager (s6, supervisord)

### Decision 9: Real-time Updates
**Choice**: WebSocket

**Considered**: SSE (Server-Sent Events)

**Rationale**:
- WebSocket gives flexibility for bidirectional communication
- Can support future features like cancel mid-stream, adjust SVI prompts live
- Works fine on RunPod (they proxy HTTP/WS)

### Decision 10: Authentication
**Choice**: No auth (self-hosted trust model)

**Rationale**:
- Self-hosted app, trust the local network
- HuggingFace and Civitai tokens are configured separately
- Avoids complexity for single-user deployments

### Decision 11: Static File Serving
**Choice**: Serve from /static folder (not embedded in binary)

**Rationale**:
- Easier to update frontend separately during development
- Containerized anyway, so single binary not critical
- Can optionally embed for release builds later

---

## Tech Stack

| Layer | Technology | Version/Notes |
|-------|------------|---------------|
| **Frontend** | React | 19.x |
| | shadcn/ui | 2.0 |
| | Tailwind CSS | 3.4.x |
| | Vite | 6.x |
| | TanStack Query | 5.x |
| | Zustand | 5.x (state management) |
| | react-canvas-masker | 1.1.x (inpainting masks) |
| **Backend** | Go | 1.23+ |
| | Chi | 5.x (router) |
| | gorilla/websocket | WebSocket handling |
| | go-sqlite3 | SQLite driver |
| | go-redis | Redis/Valkey client |
| | Bleve | 2.x (full-text search) |
| **Queue** | Valkey | 7.x (Redis fork) |
| **Downloads** | aria2 | JSON-RPC control |
| **Python** | Python | 3.10+ |
| | uv | Environment management |
| | torch | 2.5.0+ |
| | diffsynth | Vendored from infinity-mix |
| **Database** | SQLite | WAL mode |

---

## Workflows & Parameters

### Parameter Tier System

Parameters are organized into three visibility tiers:

| Tier | Visibility | Target User |
|------|------------|-------------|
| **Basic** | Always visible | Everyone |
| **Advanced** | Collapsible panel | Power users |
| **Expert** | Hidden by default | Developers/researchers |

### Workflow 1: Wan 2.2 I2V (Image-to-Video)

**Purpose**: Generate video from a single input image

**Basic Parameters**:
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| prompt | string | required | Text description |
| negative_prompt | string | "" | What to avoid |
| input_image | file | required | Starting frame |
| seed | int | random | Reproducibility |
| resolution | enum | 480x832 | Output size |
| lora | model[] | [] | LoRA selection |

**Advanced Parameters**:
| Parameter | Type | Default | Range |
|-----------|------|---------|-------|
| num_inference_steps | int | 50 | 10-100 |
| cfg_scale | float | 5.0 | 1.0-20.0 |
| num_frames | int | 81 | 4n+1 format |
| denoising_strength | float | 1.0 | 0.0-1.0 |
| camera_direction | enum | none | Left, Right, Up, Down, etc. |
| camera_speed | float | 0.0185 | 0.0-1.0+ |
| motion_bucket_id | int | auto | 0-1023 |

**Expert Parameters**:
| Parameter | Type | Default |
|-----------|------|---------|
| tiled | bool | true |
| tile_size | tuple | (30, 52) |
| tile_stride | tuple | (15, 26) |
| sigma_shift | float | 5.0 |
| switch_DiT_boundary | float | 0.875 |
| tea_cache_l1_thresh | float | null |

### Workflow 2: SVI 2.0 Pro (Infinite Video)

**Purpose**: Generate infinite-length streaming video with multiple prompts

Inherits all I2V parameters, plus:

**Basic Parameters**:
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| prompts | string[] | required | Multiple prompts for streaming |
| num_clips | int | 10 | Number of clips to generate |

**Advanced Parameters**:
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| num_motion_frames | int | 5 | Motion frames carried between clips |
| infinite_mode | bool | false | Keep generating until stopped |

**Key Concept**: SVI 2.0 Pro uses "error recycling" to maintain consistency across clips. Each clip carries forward motion latents from the previous clip.

### Workflow 3: Qwen Image Edit

**Purpose**: Edit images using natural language instructions, with inpainting support

**Basic Parameters**:
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| prompt | string | required | Edit instruction |
| edit_images | file[] | [] | One or more reference images |
| seed | int | random | Reproducibility |
| resolution | enum | 1024x1024 | Output size |
| mode | enum | "generate" | generate, edit, or inpaint |
| inpaint_mask | canvas | null | Mask from react-canvas-masker |

**Advanced Parameters**:
| Parameter | Type | Default |
|-----------|------|---------|
| num_inference_steps | int | 30 |
| cfg_scale | float | 4.0 |
| denoising_strength | float | 1.0 |
| controlnet | model | null |
| controlnet_scale | float | 1.0 |

**Important**: Qwen supports multiple input images (`edit_images` is an array). The UI needs to support adding/removing multiple reference images.

### Inpainting UI

For inpainting mode, use **react-canvas-masker** library:
- Brush size slider
- Eraser toggle
- Zoom/pan for precision
- Clear mask button
- Outputs binary mask (white = inpaint, black = preserve)

---

## Model Management

### Metadata Caching Strategy

1. **Cache everything** from HuggingFace and Civitai
2. **Filter to supported base models**: wan2.1, wan2.2, wan, qwen, qwen-image
3. **Daily sync**: Background job refreshes metadata daily
4. **On-demand refresh**: User can manually refresh specific models

### Supported Model Types

| Type | Description | Example |
|------|-------------|---------|
| checkpoint | Base models | Wan2.2-I2V-A14B |
| lora | LoRA adapters | Custom style LoRAs |
| vae | VAE variants | Wan2.2_VAE |
| controlnet | Control networks | Depth, Canny |

### Database Schema (SQLite)

```sql
CREATE TABLE models (
    id TEXT PRIMARY KEY,           -- "hf:user/model" or "civitai:12345"
    source TEXT NOT NULL,          -- "huggingface" or "civitai"
    source_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    base_model TEXT,
    author TEXT,
    description TEXT,
    tags TEXT,                     -- JSON array
    downloads INTEGER DEFAULT 0,
    rating REAL,
    nsfw INTEGER DEFAULT 0,
    files TEXT,                    -- JSON array
    thumbnail_url TEXT,
    local_path TEXT,               -- Set when downloaded
    pinned INTEGER DEFAULT 0       -- In user's config for auto-download
);
```

### Download Flow

1. User clicks "Download" on model card
2. Go backend creates aria2 download task via JSON-RPC
3. aria2 downloads file with multi-connection
4. Progress streamed to frontend via WebSocket
5. On completion, `local_path` updated in database
6. Model available for selection in workflows

---

## Configuration System

### Core Concept

The configuration system is designed for **ephemeral deployments**. Users should be able to:
1. Export their entire setup as a JSON file
2. Import that file on a new instance
3. Have all their settings restored AND pinned models auto-downloaded

### Config File Structure

```json
{
  "version": "1.0",
  "tokens": {
    "huggingface": "hf_xxxxx",
    "civitai": "xxxxx"
  },
  "defaults": {
    "i2v": {
      "num_inference_steps": 50,
      "cfg_scale": 5.0,
      "height": 480,
      "width": 832,
      "num_frames": 81
    },
    "svi": {
      "num_inference_steps": 50,
      "cfg_scale": 5.0,
      "num_motion_frames": 5,
      "clips": 10
    },
    "qwen": {
      "num_inference_steps": 30,
      "cfg_scale": 4.0,
      "height": 1024,
      "width": 1024
    }
  },
  "presets": [
    {
      "id": "cinematic-slow",
      "name": "Cinematic Slow Motion",
      "workflow": "i2v",
      "params": {
        "motion_bucket_id": 50,
        "cfg_scale": 7.0
      }
    }
  ],
  "models": {
    "base": ["hf:Wan-AI/Wan2.2-I2V-A14B"],
    "lora": ["civitai:12345", "hf:username/my-lora"],
    "controlnet": [],
    "vae": []
  }
}
```

### What's NOT in Config

- **Job history**: Not needed, the goal is quick restoration
- **Downloaded model cache**: Models are re-downloaded based on `models` field
- **Bleve search index**: Rebuilt on startup

### UI Flow

1. Browse models → Click ⭐ "Add to config" (pins the model)
2. Settings page → Click "Export Config" → Downloads JSON
3. New instance → Settings → "Import Config" → Upload JSON
4. App auto-downloads all pinned models
5. Ready to work

---

## Project Structure

```
diffbox/
├── cmd/
│   └── server/
│       └── main.go                 # Entrypoint, process supervisor
│
├── internal/
│   ├── api/                        # HTTP handlers
│   │   ├── router.go               # Chi router setup
│   │   ├── workflows.go            # POST /api/workflows/*
│   │   ├── jobs.go                 # Job CRUD
│   │   ├── models.go               # Model browser
│   │   ├── config_handlers.go      # Config export/import
│   │   └── websocket.go            # WebSocket hub
│   │
│   ├── config/                     # App configuration
│   │   └── config.go               # Environment loading
│   │
│   ├── db/                         # Database layer
│   │   └── db.go                   # SQLite + migrations
│   │
│   ├── queue/                      # Job queue
│   │   └── queue.go                # Redis Streams abstraction
│   │
│   └── worker/                     # Python worker management
│       └── manager.go              # Spawns/monitors workers
│
├── python/
│   ├── pyproject.toml              # uv project definition
│   ├── worker/
│   │   ├── __main__.py             # Worker entrypoint
│   │   ├── protocol.py             # stdin/stdout JSON protocol
│   │   ├── i2v.py                  # Wan 2.2 I2V handler
│   │   ├── svi.py                  # SVI 2.0 Pro handler
│   │   └── qwen.py                 # Qwen Image Edit handler
│   │
│   └── diffsynth/                  # Vendored from infinity-mix
│       ├── pipelines/
│       │   ├── wan_video.py
│       │   ├── wan_video_svi.py
│       │   ├── wan_video_svi_pro.py
│       │   └── qwen_image.py
│       └── ...
│
├── web/
│   ├── package.json
│   ├── vite.config.ts
│   └── src/
│       ├── App.tsx
│       ├── components/
│       │   ├── layout/Layout.tsx
│       │   ├── workflows/          # Workflow forms (to build)
│       │   ├── models/             # Model browser (to build)
│       │   └── canvas/             # Mask editor (to build)
│       └── pages/
│           ├── WorkflowPage.tsx
│           ├── ModelsPage.tsx
│           └── SettingsPage.tsx
│
├── .docs/
│   ├── ARCHITECTURE.md             # Technical architecture
│   └── HANDOVER.md                 # This file
│
├── Dockerfile
├── Makefile
└── README.md
```

---

## What's Been Built

### Scaffolding Complete (141 files)

| Component | Status | Notes |
|-----------|--------|-------|
| Go server skeleton | ✅ Built | Chi router, all endpoints stubbed |
| WebSocket hub | ✅ Built | Full implementation with subscriptions |
| SQLite database | ✅ Built | Schema, migrations, CRUD methods |
| Redis queue | ✅ Built | Streams abstraction, consumer pattern |
| Worker manager | ✅ Built | Spawns Python workers, handles protocol |
| Python protocol | ✅ Built | stdin/stdout JSON communication |
| I2V handler | ⚠️ Stubbed | Progress simulation, needs real inference |
| SVI handler | ⚠️ Stubbed | Progress simulation, needs real inference |
| Qwen handler | ⚠️ Stubbed | Progress simulation, needs real inference |
| diffsynth | ✅ Vendored | Full library from infinity-mix svi_wan22 branch |
| React app | ⚠️ Basic | Layout, pages exist, forms need work |
| Dockerfile | ✅ Built | Single container with all services |
| Makefile | ✅ Built | Build, dev, test commands |

### What Needs Implementation

1. **Wire up actual inference in Python workers**
   - Load diffsynth pipelines
   - Handle image I/O (base64 decode)
   - Implement progress callbacks
   - Save outputs properly

2. **Model management backend**
   - HuggingFace API client
   - Civitai API client
   - Bleve index creation/updates
   - aria2 JSON-RPC client

3. **Frontend completion**
   - Workflow forms with all parameter tiers
   - Image upload/preview
   - Mask editor integration
   - Model browser with search/filter
   - Download progress UI
   - Config export/import UI

4. **Process spawning**
   - Actually spawn Valkey in main.go
   - Actually spawn aria2 in main.go
   - Handle graceful shutdown

---

## Implementation Phases

### Phase 1: Foundation (MVP)
- [ ] Wire up Go server to spawn Valkey + aria2
- [ ] Implement actual I2V inference in Python worker
- [ ] Complete I2V form in React (Basic + Advanced params)
- [ ] Image upload with preview
- [ ] Job progress display with WebSocket
- [ ] Output video display/download
- [ ] Docker image that runs on RunPod

**Exit criteria**: Can upload image, generate video, download result

### Phase 2: All Workflows
- [ ] SVI 2.0 Pro workflow (multi-prompt UI)
- [ ] Qwen Image Edit workflow
- [ ] Mask editor with react-canvas-masker
- [ ] Multi-image input for Qwen
- [ ] Preset system (save/load parameter sets)

**Exit criteria**: All three workflows functional

### Phase 3: Model Management
- [ ] HuggingFace API client
- [ ] Civitai API client
- [ ] Bleve FTS index
- [ ] Model browser UI with search/filter
- [ ] Model cards with download button
- [ ] aria2 download integration
- [ ] Download progress via WebSocket
- [ ] Pin models to config

**Exit criteria**: Can browse, search, download models from HF/Civitai

### Phase 4: Config & Polish
- [ ] Config export/import
- [ ] Auto-download on config import
- [ ] Token management UI
- [ ] Expert parameter panels
- [ ] Error handling and retry logic
- [ ] ControlNet support
- [ ] Loading states and skeletons

**Exit criteria**: Production-ready for self-hosting

---

## Key Files Reference

### Go Backend

| File | Purpose |
|------|---------|
| `cmd/server/main.go` | Entrypoint, spawns services |
| `internal/api/router.go` | All route definitions |
| `internal/api/workflows.go` | Job submission handlers |
| `internal/api/websocket.go` | WebSocket hub + protocol |
| `internal/db/db.go` | SQLite schema + methods |
| `internal/queue/queue.go` | Redis Streams wrapper |
| `internal/worker/manager.go` | Python worker lifecycle |

### Python Worker

| File | Purpose |
|------|---------|
| `python/worker/__main__.py` | Worker loop, job dispatch |
| `python/worker/protocol.py` | JSON message helpers |
| `python/worker/i2v.py` | Wan 2.2 I2V inference |
| `python/worker/svi.py` | SVI 2.0 Pro inference |
| `python/worker/qwen.py` | Qwen image editing |

### Frontend

| File | Purpose |
|------|---------|
| `web/src/App.tsx` | Route definitions |
| `web/src/components/layout/Layout.tsx` | App shell + nav |
| `web/src/pages/WorkflowPage.tsx` | Workflow selector + forms |
| `web/src/pages/ModelsPage.tsx` | Model browser |
| `web/src/pages/SettingsPage.tsx` | Tokens + config |

### diffsynth (Vendored)

| File | Purpose |
|------|---------|
| `python/diffsynth/pipelines/wan_video.py` | Base Wan pipeline |
| `python/diffsynth/pipelines/wan_video_svi.py` | SVI pipeline |
| `python/diffsynth/pipelines/wan_video_svi_pro.py` | SVI Pro pipeline |
| `python/diffsynth/pipelines/qwen_image.py` | Qwen pipeline |
| `python/diffsynth/configs/model_configs.py` | Model loader configs |

---

## External Dependencies

### APIs

| Service | Purpose | Auth |
|---------|---------|------|
| HuggingFace | Model metadata, downloads | Token (optional for public) |
| Civitai | Model metadata, downloads | API key |

### System Dependencies (Dockerfile)

| Dependency | Purpose |
|------------|---------|
| CUDA 12.1 | GPU inference |
| aria2 | Multi-connection downloads |
| ffmpeg | Video encoding |
| uv | Python environment |
| Valkey | Redis-compatible queue |

### Go Dependencies

```go
github.com/go-chi/chi/v5        // Router
github.com/gorilla/websocket    // WebSocket
github.com/mattn/go-sqlite3     // SQLite
github.com/redis/go-redis/v9    // Valkey client
github.com/blevesearch/bleve/v2 // Full-text search
github.com/google/uuid          // Job IDs
```

### Python Dependencies

```
torch>=2.5.0
torchvision
transformers>=4.46.0
accelerate
safetensors
peft>=0.17.0
imageio[ffmpeg]
pillow
```

### Frontend Dependencies

```
react@19
react-router-dom@7
@tanstack/react-query@5
zustand@5
react-canvas-masker
tailwindcss
```

---

## Design Decisions & Rationale

### Why vendor diffsynth?

The diffsynth library from infinity-mix (svi_wan22 branch) contains:
- SVI 2.0 Pro specific pipelines not in upstream DiffSynth-Studio
- Wan 2.2 model support
- All required model loaders and converters

Vendoring ensures stability and allows modifications without upstream changes.

### Why Go spawns Python (not the other way around)?

- Go is better at HTTP serving, WebSocket management, process supervision
- Python is better at ML inference with existing torch ecosystem
- Clean separation of concerns
- Go binary is the single entrypoint (simpler container)

### Why Redis Streams over alternatives?

- Persistent (survives restarts)
- Ordered (FIFO by default)
- Consumer groups (for scaling workers later)
- Native progress/status updates via same channel
- Valkey is Redis-compatible and fully open source

### Why react-canvas-masker for inpainting?

- Specifically designed for AI inpainting workflows
- React 18+ compatible, hook-first API
- Zoom/pan for precision editing
- Outputs clean binary masks
- Actively maintained (Aug 2025)

### Why single container?

- RunPod and Vast.ai work best with single containers
- Simpler deployment (one image to push)
- No Docker Compose complexity
- Go handles service supervision

---

## Getting Started (For New Developer)

### 1. Clone and Setup

```bash
# Get the scaffold
git clone -b claude/diffbox-full-scaffold-wo3f7 \
  https://github.com/druarnfield/infinity-mix.git temp
mv temp/diffbox/* ./your-diffbox-repo/
cd your-diffbox-repo

# Initialize
make init  # Installs Go deps, Python deps, npm packages
```

### 2. Run Locally

```bash
# Terminal 1: Start Go server
make dev

# Terminal 2: Start frontend dev server
cd web && npm run dev
```

### 3. First Implementation Task

Start with Phase 1: Wire up actual I2V inference

1. Open `python/worker/i2v.py`
2. Implement `_load_pipeline()` using diffsynth
3. Implement actual inference in `run()`
4. Test with `make python-worker` directly
5. Then test full flow through Go

### 4. Key Commands

```bash
make build      # Build Go binary
make dev        # Run with hot reload
make frontend   # Build React app
make docker     # Build Docker image
make test       # Run all tests
```

---

## Questions? Context?

This handover was created from a conversation covering:
- Project vision and use case (self-hosted, ephemeral cloud)
- All architectural decisions with rationale
- Complete parameter research for all three workflows
- Model management strategy with HF/Civitai sync
- Implementation phasing

The scaffold in `diffbox/` is ready for implementation. Start with Phase 1 to get a working end-to-end flow, then iterate.

Good luck! 🚀
