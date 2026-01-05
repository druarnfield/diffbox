# I2V + Qwen Workflows + CI/CD Design

**Date:** 2026-01-05
**Status:** Approved

## Overview

This design covers the first deliverable milestone for diffbox: two working inference workflows (I2V and Qwen Image Edit) with CI/CD pipeline for RunPod deployment.

## Scope

### In Scope

1. **CI/CD Pipeline** - GitHub Actions workflow that builds and pushes Docker images to GHCR on push to main
2. **I2V Workflow (Basic)** - Image-to-video generation with minimal parameters
3. **Qwen Workflow (Basic)** - Instruction-based image editing with 3 image inputs, no mask editor
4. **Model Auto-Download** - Required models download on startup via aria2

### Out of Scope

- Advanced/Expert parameter tiers (UI designed for expansion, not implemented)
- SVI workflow (infinite video)
- Qwen mask editor / inpainting
- Model browser UI (auto-download only)
- Config export/import
- Token management UI (env vars only)

## Success Criteria

- [ ] Push to main triggers automated build
- [ ] Image appears at `ghcr.io/<owner>/diffbox:latest`
- [ ] RunPod pod can pull and run the image
- [ ] First startup downloads required models automatically
- [ ] **I2V:** Upload image + enter prompt → get video
- [ ] **Qwen:** Upload 3 images + enter instruction → get edited image
- [ ] Progress updates flow via WebSocket for both
- [ ] Outputs are viewable and downloadable

## Architecture

### System Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                     Docker Container                            │
│                                                                 │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐  │
│  │  React   │───▶│  Go API  │───▶│  Valkey  │───▶│  Python  │  │
│  │   UI     │◀───│  + WS    │◀───│ (Redis)  │◀───│  Worker  │  │
│  └──────────┘    └──────────┘    └──────────┘    └──────────┘  │
│                       │                               │         │
│                       ▼                               ▼         │
│                  ┌──────────┐                   ┌──────────┐    │
│                  │  SQLite  │                   │ diffsynth│    │
│                  │   (jobs) │                   │ (models) │    │
│                  └──────────┘                   └──────────┘    │
│                                                                 │
│  Startup: aria2 downloads models if missing                     │
└─────────────────────────────────────────────────────────────────┘
```

### Startup Sequence

1. Go server starts as PID 1
2. Spawns Valkey (Redis)
3. Spawns aria2 daemon
4. Checks for required models → downloads missing via aria2
5. Starts HTTP server + WebSocket hub
6. Ready to accept jobs

### Job Flow

1. User submits via UI → `POST /api/workflows/{i2v,qwen}`
2. Go creates job in SQLite, pushes to Redis Stream
3. Python worker pulls job, loads pipeline, runs inference
4. Worker emits progress JSON → Go → WebSocket → UI
5. Output saved to `/outputs`, job marked complete

## CI/CD Pipeline

**File:** `.github/workflows/build.yml`

**Trigger:** Push to `main` branch

### Pipeline Stages

```
┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐
│  Lint   │───▶│  Test   │───▶│  Build  │───▶│  Push   │
│         │    │         │    │ Docker  │    │  GHCR   │
└─────────┘    └─────────┘    └─────────┘    └─────────┘
```

| Stage | Commands | Fails if |
|-------|----------|----------|
| Lint | `make lint` | Go, Python, or TS lint errors |
| Test | `make test` | Go or Python tests fail |
| Build | `docker build -t diffbox .` | Dockerfile errors |
| Push | Push to `ghcr.io/<owner>/diffbox` | Auth issues |

### Image Tags

- `latest` - always points to most recent main
- `sha-<first 7 chars>` - immutable reference to specific commit

### Secrets

None required for GHCR with public repo (uses `GITHUB_TOKEN` automatically)

### Runner

`ubuntu-latest` (no GPU needed - just building the image)

## Model Auto-Download

### Environment Variables

- `HF_TOKEN` - HuggingFace access token
- `CIVITAI_TOKEN` - Civitai API token (if needed)

### Wan 2.2 I2V A14B (BF16)

**Source:** [Comfy-Org/Wan_2.2_ComfyUI_Repackaged](https://huggingface.co/Comfy-Org/Wan_2.2_ComfyUI_Repackaged)

| Component | File | Size |
|-----------|------|------|
| High noise DiT | `split_files/diffusion_models/wan2.2_i2v_high_noise_14B_fp16.safetensors` | 28.6 GB |
| Low noise DiT | `split_files/diffusion_models/wan2.2_i2v_low_noise_14B_fp16.safetensors` | 28.6 GB |
| T5 text encoder | `split_files/text_encoders/umt5_xxl_fp16.safetensors` | 11.4 GB |
| VAE | `split_files/vae/wan_2.1_vae.safetensors` | 254 MB |

**Subtotal:** ~69 GB

### Qwen Image Edit 2511 (BF16)

**Source:** [Comfy-Org/Qwen-Image-Edit_ComfyUI](https://huggingface.co/Comfy-Org/Qwen-Image-Edit_ComfyUI) + [Comfy-Org/Qwen-Image_ComfyUI](https://huggingface.co/Comfy-Org/Qwen-Image_ComfyUI)

| Component | File | Size |
|-----------|------|------|
| Diffusion model | `split_files/diffusion_models/qwen_image_edit_2511_bf16.safetensors` | 40.9 GB |
| Text encoder | `split_files/text_encoders/qwen_2.5_vl_7b.safetensors` | 16.6 GB |
| VAE | `split_files/vae/qwen_image_vae.safetensors` | 254 MB |

**Subtotal:** ~58 GB

### Total Download

**~127 GB**

### Download Behavior

- First startup checks `/models` directory
- Missing files queued to aria2 with HF_TOKEN auth header
- Parallel multi-connection downloads
- Server blocks until all models ready
- Progress logged to stdout (visible in RunPod logs)

## Workflow UI (Basic Tier)

### I2V Form (Image-to-Video)

| Field | Type | Required |
|-------|------|----------|
| Image | File upload (drag & drop) | Yes |
| Prompt | Textarea | Yes |
| Generate | Button | - |

**Output:** Video player with download button

### Qwen Form (Image Edit)

| Field | Type | Required |
|-------|------|----------|
| Image 1 | File upload | Yes |
| Image 2 | File upload | Optional |
| Image 3 | File upload | Optional |
| Instruction | Textarea | Yes |
| Generate | Button | - |

**Output:** Image with download button

### Shared UI Elements

- Progress bar with percentage (via WebSocket)
- Job queue showing pending/running/completed jobs
- Cancel button for running jobs
- Output gallery for completed jobs

### Expansion Path

Forms use collapsible "Advanced" section (hidden by default) for future parameters like seed, guidance scale, steps.

### Implementation Notes

- Use `frontend-design` skill for UI implementation
- Use shadcn MCP for components
- Use Flowbite components where available

## Implementation Order

```
1. CI/CD Pipeline
   └── Can test/deploy everything after

2. Model Auto-Download
   └── Startup downloads required models
   └── Blocks until ready

3. I2V Workflow (vertical slice)
   ├── API endpoint (POST /api/workflows/i2v)
   ├── Python worker wiring (load pipeline, run inference)
   ├── WebSocket progress updates
   ├── Form UI
   └── Output handling (video player, download)

4. Qwen Workflow (leverage I2V patterns)
   ├── API endpoint (POST /api/workflows/qwen)
   ├── Python worker wiring
   └── Form UI (3 image uploads)
```

### Rationale

- CI/CD first = can test on RunPod immediately
- Model download before workflows = inference won't fail on missing models
- I2V before Qwen = establishes patterns, Qwen reuses 80% of the code
