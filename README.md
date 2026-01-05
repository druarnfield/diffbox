# diffbox

A self-hosted web application for AI video and image generation, optimized for ephemeral cloud deployments (RunPod, Vast.ai).

## Features

- **Wan 2.2 I2V** - Image-to-Video generation
- **Wan 2.2 SVI 2.0 Pro** - Infinite/streaming video generation
- **Qwen Image Edit** - Instruction-based image editing with inpainting
- **Model Browser** - Search and download from HuggingFace & Civitai
- **Config Export/Import** - Quick setup on ephemeral instances

## Quick Start

### RunPod / Vast.ai

```bash
# Pull and run the Docker image
docker run --gpus all -p 8080:8080 \
  -v /workspace/models:/models \
  -v /workspace/data:/data \
  -v /workspace/outputs:/outputs \
  ghcr.io/druarnfield/diffbox:latest
```

### Local Development

```bash
# Clone the repo
git clone https://github.com/druarnfield/diffbox.git
cd diffbox

# Initialize development environment
make init

# Run in development mode
make dev
```

## Architecture

See [.docs/ARCHITECTURE.md](.docs/ARCHITECTURE.md) for detailed architecture documentation.

### Tech Stack

| Layer | Technology |
|-------|------------|
| Frontend | React + shadcn/ui 2.0 |
| Backend | Go + Chi |
| Queue | Valkey (Redis) + Streams |
| Database | SQLite |
| Search | Bleve |
| Downloads | aria2 |
| Python | uv + diffsynth |

## Configuration

### Environment Variables

```bash
DIFFBOX_PORT=8080
DIFFBOX_DATA_DIR=/data
DIFFBOX_MODELS_DIR=/models
DIFFBOX_OUTPUTS_DIR=/outputs
```

### Config File

Export/import your settings via the UI or API:

```json
{
  "version": "1.0",
  "tokens": {
    "huggingface": "hf_xxx",
    "civitai": "xxx"
  },
  "models": {
    "base": ["hf:Wan-AI/Wan2.2-I2V-A14B"],
    "lora": ["civitai:12345"]
  }
}
```

## API

### REST Endpoints

```
POST /api/workflows/i2v     - Submit I2V job
POST /api/workflows/svi     - Submit SVI job
POST /api/workflows/qwen    - Submit Qwen job
GET  /api/jobs              - List jobs
GET  /api/models            - Search models
POST /api/config            - Import config
```

### WebSocket

Connect to `/ws` for real-time job progress updates.

## License

MIT
