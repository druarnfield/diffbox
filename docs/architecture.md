# diffbox Architecture

> A self-hosted web application for AI video and image generation, optimized for ephemeral cloud deployments (RunPod, Vast.ai).

## Overview

diffbox provides a clean, opinionated interface for three workflows:
1. **Wan 2.2 I2V** - Image-to-Video generation
2. **Wan 2.2 SVI 2.0 Pro** - Infinite/streaming video generation
3. **Qwen Image Edit** - Instruction-based image editing with inpainting

## Tech Stack

| Layer | Technology | Purpose |
|-------|------------|---------|
| **Frontend** | React + shadcn/ui 2.0 | Modern, accessible UI components |
| **Backend** | Go + Chi router | API server, process supervisor |
| **Job Queue** | Valkey + Redis Streams | Async job processing, progress streaming |
| **Database** | SQLite | Job metadata, model cache |
| **Search** | Bleve | Full-text search for model browser |
| **Downloads** | aria2 (JSON-RPC) | Fast multi-connection downloads |
| **Python** | uv + diffsynth | Inference workers |
| **Deployment** | Single Docker image | RunPod/Vast.ai compatible |

## Project Structure

```
diffbox/
├── cmd/
│   └── server/
│       └── main.go                 # Entrypoint - spawns Valkey, aria2, workers
│
├── internal/
│   ├── api/                        # HTTP API layer
│   │   ├── router.go               # Chi router setup
│   │   ├── middleware.go           # Logging, recovery, etc.
│   │   ├── workflows.go            # POST /api/workflows/{type}
│   │   ├── jobs.go                 # GET /api/jobs, DELETE /api/jobs/{id}
│   │   ├── models.go               # GET /api/models, POST /api/downloads
│   │   ├── config.go               # GET/POST /api/config (export/import)
│   │   └── ws.go                   # WebSocket handler for progress
│   │
│   ├── queue/                      # Job queue abstraction
│   │   ├── queue.go                # Interface
│   │   ├── redis.go                # Redis Streams implementation
│   │   └── consumer.go             # Job consumer (dispatches to Python)
│   │
│   ├── db/                         # Data persistence
│   │   ├── db.go                   # SQLite connection
│   │   ├── models.go               # Model metadata table
│   │   ├── jobs.go                 # Job history table
│   │   └── migrations/             # SQL migrations
│   │
│   ├── search/                     # Full-text search
│   │   ├── index.go                # Bleve index management
│   │   └── models.go               # Model document indexing
│   │
│   ├── sync/                       # External API sync
│   │   ├── sync.go                 # Sync orchestrator
│   │   ├── huggingface.go          # HF API client
│   │   └── civitai.go              # Civitai API client
│   │
│   ├── downloader/                 # aria2 integration
│   │   ├── aria2.go                # JSON-RPC client
│   │   └── manager.go              # Download queue management
│   │
│   ├── worker/                     # Python worker management
│   │   ├── manager.go              # Spawns/monitors Python workers
│   │   └── protocol.go             # JSON protocol for Go<->Python
│   │
│   └── config/                     # Configuration
│       ├── config.go               # Config struct & loading
│       └── export.go               # Import/export logic
│
├── python/
│   ├── pyproject.toml              # uv project definition
│   ├── worker/
│   │   ├── __main__.py             # Worker entrypoint
│   │   ├── protocol.py             # JSON protocol handling
│   │   ├── i2v.py                  # Wan 2.2 I2V handler
│   │   ├── svi.py                  # SVI 2.0 Pro handler
│   │   └── qwen.py                 # Qwen Image Edit handler
│   └── diffsynth/                  # Vendored from infinity-mix fork
│
├── web/
│   ├── package.json
│   ├── vite.config.ts
│   ├── src/
│   │   ├── main.tsx
│   │   ├── App.tsx
│   │   ├── components/
│   │   │   ├── ui/                 # shadcn/ui components
│   │   │   ├── layout/
│   │   │   │   ├── Sidebar.tsx
│   │   │   │   └── Header.tsx
│   │   │   ├── workflows/
│   │   │   │   ├── I2VForm.tsx
│   │   │   │   ├── SVIForm.tsx
│   │   │   │   └── QwenForm.tsx
│   │   │   ├── models/
│   │   │   │   ├── ModelBrowser.tsx
│   │   │   │   ├── ModelCard.tsx
│   │   │   │   └── DownloadManager.tsx
│   │   │   ├── jobs/
│   │   │   │   ├── JobList.tsx
│   │   │   │   └── JobProgress.tsx
│   │   │   ├── canvas/
│   │   │   │   └── MaskEditor.tsx  # react-canvas-masker wrapper
│   │   │   └── config/
│   │   │       ├── ConfigExport.tsx
│   │   │       └── TokenSettings.tsx
│   │   ├── hooks/
│   │   │   ├── useWebSocket.ts
│   │   │   ├── useJob.ts
│   │   │   └── useModels.ts
│   │   ├── lib/
│   │   │   ├── api.ts              # API client
│   │   │   └── utils.ts
│   │   └── pages/
│   │       ├── WorkflowPage.tsx
│   │       ├── ModelsPage.tsx
│   │       └── SettingsPage.tsx
│   └── dist/                       # Built output → served from /static
│
├── data/                           # Runtime data (gitignored)
│   ├── diffbox.db                  # SQLite database
│   └── search.bleve/               # Bleve index
│
├── models/                         # Downloaded models (gitignored)
│   ├── base/
│   ├── lora/
│   ├── controlnet/
│   └── vae/
│
├── outputs/                        # Generated content (gitignored)
│
├── .docs/                          # Documentation
│   └── ARCHITECTURE.md             # This file
│
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

## Component Architecture

### Process Model

```
┌─────────────────────────────────────────────────────────────────┐
│                     DOCKER CONTAINER                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐       │
│   │  Go Server  │────▶│   Valkey    │◀────│   aria2     │       │
│   │  (PID 1)    │     │  (Redis)    │     │  (daemon)   │       │
│   └──────┬──────┘     └──────┬──────┘     └─────────────┘       │
│          │                   │                                   │
│          │ spawns            │ Redis Streams                     │
│          ▼                   ▼                                   │
│   ┌─────────────────────────────────────┐                       │
│   │         Python Workers (uv)          │                       │
│   │  ┌─────────┐ ┌─────────┐ ┌────────┐ │                       │
│   │  │  I2V    │ │   SVI   │ │  Qwen  │ │                       │
│   │  │ Worker  │ │ Worker  │ │ Worker │ │                       │
│   │  └─────────┘ └─────────┘ └────────┘ │                       │
│   └─────────────────────────────────────┘                       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Request Flow

```
┌────────┐     ┌────────────┐     ┌─────────┐     ┌────────────┐
│ React  │────▶│  Go API    │────▶│ Valkey  │────▶│  Python    │
│   UI   │ WS  │  Server    │     │ Stream  │     │  Worker    │
└────────┘◀────└────────────┘◀────└─────────┘◀────└────────────┘
           progress updates      job result      inference
```

1. User submits job via REST API
2. Go server validates, creates job record, pushes to Redis Stream
3. Python worker consumes job, runs inference
4. Worker publishes progress updates to Redis Stream
5. Go server relays progress to client via WebSocket
6. Worker publishes result, Go server notifies client

### Data Flow: Model Downloads

```
┌────────┐     ┌────────────┐     ┌─────────┐     ┌────────────┐
│ React  │────▶│  Go API    │────▶│  aria2  │────▶│  HF/Civit  │
│   UI   │     │  Server    │     │ daemon  │     │   CDN      │
└────────┘◀────└────────────┘◀────└─────────┘     └────────────┘
           progress via WS      JSON-RPC
```

## API Design

### REST Endpoints

```
# Workflows
POST   /api/workflows/i2v          Submit I2V job
POST   /api/workflows/svi          Submit SVI job
POST   /api/workflows/qwen         Submit Qwen job

# Jobs
GET    /api/jobs                   List jobs (with pagination)
GET    /api/jobs/:id               Get job details
DELETE /api/jobs/:id               Cancel job

# Models
GET    /api/models                 Search models (query, type, base)
GET    /api/models/:source/:id     Get model details
POST   /api/models/:source/:id/download    Start download
DELETE /api/models/:source/:id     Remove downloaded model
GET    /api/models/local           List locally available models

# Downloads
GET    /api/downloads              List active downloads
DELETE /api/downloads/:id          Cancel download

# Config
GET    /api/config                 Export config as JSON
POST   /api/config                 Import config JSON
GET    /api/config/tokens          Get token status (not values)
PUT    /api/config/tokens          Update tokens

# Health
GET    /api/health                 Health check
```

### WebSocket Protocol

```
Connect: ws://host/ws

# Server → Client messages
{
  "type": "job:progress",
  "job_id": "xxx",
  "progress": 0.45,
  "stage": "Denoising step 23/50",
  "preview": "base64..."          // Optional preview frame
}

{
  "type": "job:complete",
  "job_id": "xxx",
  "output": {
    "type": "video",
    "path": "/outputs/xxx.mp4",
    "frames": 81
  }
}

{
  "type": "job:error",
  "job_id": "xxx",
  "error": "CUDA out of memory"
}

{
  "type": "download:progress",
  "download_id": "xxx",
  "progress": 0.67,
  "speed": "125 MB/s"
}

# Client → Server messages
{
  "type": "subscribe",
  "job_ids": ["xxx", "yyy"]
}

{
  "type": "unsubscribe",
  "job_ids": ["xxx"]
}
```

## Configuration

### User Config (diffbox-config.json)

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
    "base": [
      "hf:Wan-AI/Wan2.2-I2V-A14B"
    ],
    "lora": [
      "civitai:12345",
      "hf:username/my-lora"
    ],
    "controlnet": [],
    "vae": []
  }
}
```

### Environment Variables

```bash
# Server
DIFFBOX_PORT=8080
DIFFBOX_DATA_DIR=/data
DIFFBOX_MODELS_DIR=/models
DIFFBOX_OUTPUTS_DIR=/outputs

# Valkey
DIFFBOX_VALKEY_PORT=6379

# aria2
DIFFBOX_ARIA2_PORT=6800
DIFFBOX_ARIA2_MAX_CONNECTIONS=16

# Python
DIFFBOX_WORKER_COUNT=1
DIFFBOX_PYTHON_PATH=/app/python
```

## Workflow Parameters

### Tier System

Parameters are exposed in three tiers:

**Basic** - Always visible
- prompt, negative_prompt
- seed
- resolution presets
- LoRA selection

**Advanced** - Collapsible panel
- num_inference_steps
- cfg_scale
- denoising_strength
- camera_control (I2V)
- motion_frames (SVI)
- inpainting options (Qwen)

**Expert** - Hidden by default
- sigma_shift
- switch_DiT_boundary
- VAE tiling options
- TeaCache settings

### Workflow: Wan 2.2 I2V

| Parameter | Type | Default | Tier |
|-----------|------|---------|------|
| prompt | string | required | Basic |
| negative_prompt | string | "" | Basic |
| input_image | file | required | Basic |
| seed | int | random | Basic |
| resolution | enum | 480x832 | Basic |
| lora | model[] | [] | Basic |
| num_inference_steps | int | 50 | Advanced |
| cfg_scale | float | 5.0 | Advanced |
| num_frames | int | 81 | Advanced |
| denoising_strength | float | 1.0 | Advanced |
| camera_direction | enum | none | Advanced |
| camera_speed | float | 0.0185 | Advanced |
| motion_bucket_id | int | auto | Advanced |
| tiled | bool | true | Expert |
| tile_size | int[] | [30,52] | Expert |
| sigma_shift | float | 5.0 | Expert |

### Workflow: SVI 2.0 Pro

Inherits all I2V parameters, plus:

| Parameter | Type | Default | Tier |
|-----------|------|---------|------|
| prompts | string[] | required | Basic |
| num_clips | int | 10 | Basic |
| num_motion_frames | int | 5 | Advanced |
| infinite_mode | bool | false | Advanced |

### Workflow: Qwen Image Edit

| Parameter | Type | Default | Tier |
|-----------|------|---------|------|
| prompt | string | required | Basic |
| edit_images | file[] | [] | Basic |
| seed | int | random | Basic |
| resolution | enum | 1024x1024 | Basic |
| mode | enum | edit | Basic |
| inpaint_mask | canvas | null | Basic (if inpaint) |
| num_inference_steps | int | 30 | Advanced |
| cfg_scale | float | 4.0 | Advanced |
| denoising_strength | float | 1.0 | Advanced |
| controlnet | model | null | Advanced |
| controlnet_scale | float | 1.0 | Advanced |
| tiled | bool | false | Expert |

## Model Management

### Metadata Schema (SQLite)

```sql
CREATE TABLE models (
    id TEXT PRIMARY KEY,           -- "hf:user/model" or "civitai:12345"
    source TEXT NOT NULL,          -- "huggingface" or "civitai"
    source_id TEXT NOT NULL,       -- Original ID from source
    name TEXT NOT NULL,
    type TEXT NOT NULL,            -- "checkpoint", "lora", "vae", "controlnet"
    base_model TEXT,               -- "wan2.2", "qwen", "sdxl", etc.
    author TEXT,
    description TEXT,
    tags TEXT,                     -- JSON array
    downloads INTEGER DEFAULT 0,
    rating REAL,
    nsfw BOOLEAN DEFAULT FALSE,
    files TEXT,                    -- JSON array of file info
    thumbnail_url TEXT,
    created_at DATETIME,
    updated_at DATETIME,
    synced_at DATETIME,

    -- Local status
    local_path TEXT,               -- Path if downloaded
    local_size INTEGER,            -- Size in bytes
    downloaded_at DATETIME,
    pinned BOOLEAN DEFAULT FALSE   -- In user's config
);

CREATE INDEX idx_models_type ON models(type);
CREATE INDEX idx_models_base ON models(base_model);
CREATE INDEX idx_models_local ON models(local_path) WHERE local_path IS NOT NULL;
```

### Sync Strategy

1. **Initial sync**: Paginate through HF/Civitai APIs, filter to supported base models
2. **Daily sync**: Cron job updates metadata, new models
3. **On-demand**: User can trigger refresh for specific model

### Supported Base Models (Filter)

```go
var supportedBaseModels = []string{
    // Video
    "wan2.1", "wan2.2", "wan",

    // Image
    "qwen", "qwen-image",
}
```

## Deployment

### Dockerfile

```dockerfile
FROM nvidia/cuda:12.1-runtime-ubuntu22.04

# Install system deps
RUN apt-get update && apt-get install -y \
    aria2 \
    && rm -rf /var/lib/apt/lists/*

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# Install Valkey
COPY --from=valkey/valkey:7-alpine /usr/local/bin/valkey-server /usr/local/bin/

# Copy Go binary
COPY diffbox /usr/local/bin/diffbox

# Copy Python worker
COPY python /app/python
WORKDIR /app/python
RUN uv sync

# Copy frontend
COPY web/dist /app/static

# Volumes
VOLUME ["/data", "/models", "/outputs"]

# Ports
EXPOSE 8080

# Entrypoint
ENTRYPOINT ["/usr/local/bin/diffbox"]
```

### RunPod Template

```json
{
  "name": "diffbox",
  "imageName": "ghcr.io/username/diffbox:latest",
  "volumeInGb": 50,
  "volumeMountPath": "/workspace",
  "ports": "8080/http",
  "env": {
    "DIFFBOX_MODELS_DIR": "/workspace/models",
    "DIFFBOX_DATA_DIR": "/workspace/data"
  }
}
```

## Implementation Phases

### Phase 1: Foundation (MVP)
- [ ] Go server skeleton (Chi, SQLite, Valkey spawn)
- [ ] Python worker with uv (I2V workflow only)
- [ ] Basic React UI (job submit, progress WebSocket)
- [ ] Manual model path configuration
- [ ] Docker image running on RunPod

### Phase 2: All Workflows
- [ ] SVI 2.0 Pro workflow + multi-prompt UI
- [ ] Qwen Image Edit workflow
- [ ] Mask editor (react-canvas-masker)
- [ ] Multi-image input for Qwen
- [ ] Preset system (save/load)

### Phase 3: Model Management
- [ ] HF + Civitai API clients
- [ ] Bleve FTS index
- [ ] Model browser UI (search, filter, cards)
- [ ] aria2 download integration
- [ ] Download progress via WebSocket

### Phase 4: Config & Polish
- [ ] Config export/import
- [ ] Auto-download on config load
- [ ] Token management UI
- [ ] Advanced/Expert parameter panels
- [ ] Error handling, retry logic
- [ ] ControlNet support

## Dependencies

### Go

```go
require (
    github.com/go-chi/chi/v5
    github.com/mattn/go-sqlite3
    github.com/blevesearch/bleve/v2
    github.com/gorilla/websocket
    github.com/redis/go-redis/v9
)
```

### Python

```toml
[project]
dependencies = [
    "torch>=2.5.0",
    "torchvision",
    "transformers>=4.46.0",
    "accelerate",
    "safetensors",
    "einops",
    "imageio[ffmpeg]",
    "pillow",
    "numpy",
]
```

### Frontend

```json
{
  "dependencies": {
    "react": "^19.0.0",
    "@tanstack/react-query": "^5.0.0",
    "react-router-dom": "^7.0.0",
    "zustand": "^5.0.0",
    "react-canvas-masker": "^1.1.0"
  }
}
```
