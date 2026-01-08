# RunPod Deployment Guide

This guide covers deploying diffbox to RunPod as a single optimized container with ComfyUI integrated.

## Architecture

The RunPod deployment uses a **single Docker container** running both services:
- **ComfyUI** (port 8188) - Handles inference via API
- **diffbox** (port 8080) - Main application server

Both services are managed by **supervisord** and share the same GPU.

## Prerequisites

- RunPod account
- Docker installed locally (for building)
- Docker Hub or container registry account

## Building the Container

### 1. Build the RunPod Image

```bash
# Build the optimized RunPod image
docker build -f Dockerfile.runpod -t your-username/diffbox-runpod:latest .

# Test locally (requires GPU)
docker run --rm --gpus all \
  -p 8080:8080 \
  -p 8188:8188 \
  -v $(pwd)/models:/models \
  -v $(pwd)/outputs:/outputs \
  your-username/diffbox-runpod:latest

# Push to registry
docker push your-username/diffbox-runpod:latest
```

### 2. Deploy to RunPod

1. Log in to [RunPod](https://www.runpod.io/)
2. Navigate to **My Pods** → **+ Deploy**
3. Select GPU type (recommend RTX 4090 or A100 40GB)
4. Choose **Custom Container**
5. Enter image: `your-username/diffbox-runpod:latest`
6. Configure:
   - **Container Disk**: 50GB minimum (for models)
   - **Expose HTTP Ports**: 8080
   - **Expose TCP Ports**: 8188 (optional, for debugging)
7. Add Volume Mounts:
   - `/workspace/models` → `/models` (persistent model storage)
   - `/workspace/outputs` → `/outputs` (persistent outputs)
   - `/workspace/data` → `/data` (persistent database)
8. Click **Deploy**

## First-Time Setup

After deployment, the container will:

1. **Start ComfyUI** (takes ~30 seconds)
2. **Start diffbox** server
3. **Download models** via aria2 (~100GB, takes 1-2 hours on first run)
   - Wan 2.2 I2V models (~60GB)
   - Qwen Image Edit models (~40GB)
   - Dolphin-Mistral chat model (~49GB)

### Monitor Initial Setup

Connect to the pod via SSH or web terminal:

```bash
# Watch diffbox logs
tail -f /var/log/diffbox/stdout.log

# Watch ComfyUI logs
tail -f /var/log/comfyui/stdout.log

# Check supervisor status
supervisorctl status

# Check model download progress
ls -lh /models/
```

## Environment Variables

Configure via RunPod environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DIFFBOX_PORT` | `8080` | Main server port |
| `DIFFBOX_MODELS_DIR` | `/models` | Model storage path |
| `DIFFBOX_OUTPUTS_DIR` | `/outputs` | Generated outputs |
| `DIFFBOX_DATA_DIR` | `/data` | SQLite database |
| `COMFYUI_URL` | `http://localhost:8188` | ComfyUI endpoint |
| `HF_TOKEN` | (none) | HuggingFace token for gated models |

## Accessing the Application

Once deployed, access diffbox at:
```
https://[pod-id]-8080.proxy.runpod.net
```

ComfyUI debug interface (optional):
```
https://[pod-id]-8188.proxy.runpod.net
```

## Troubleshooting

### ComfyUI Won't Start

```bash
# Check ComfyUI logs
tail -n 100 /var/log/comfyui/stderr.log

# Manually test ComfyUI
supervisorctl stop comfyui
cd /app/comfyui
python3 main.py --listen 0.0.0.0 --port 8188
```

### diffbox Can't Connect to ComfyUI

```bash
# Verify ComfyUI is running
curl http://localhost:8188

# Check diffbox logs
tail -n 100 /var/log/diffbox/stderr.log

# Restart diffbox
supervisorctl restart diffbox
```

### Models Not Downloading

```bash
# Check aria2 process
ps aux | grep aria2

# Manual model download
aria2c -x 16 -s 16 https://huggingface.co/[model-url]
```

### Out of VRAM

The default configuration targets **24GB VRAM**. For GPUs with less:

1. Edit `/app/comfyui/extra_model_paths.yaml` to enable model offloading
2. Reduce concurrent workers in diffbox config
3. Use smaller model variants (TI2V-5B instead of I2V-14B)

## Performance Optimization

### Model Caching

Models persist in `/workspace/models` across pod restarts. Ensure this is mounted to a persistent volume.

### VRAM Management

ComfyUI handles VRAM automatically via model offloading. For extreme memory constraints:

```bash
# Enable aggressive offloading in ComfyUI
export COMFYUI_ARGS="--lowvram --disable-smart-memory"
supervisorctl restart comfyui
```

### Concurrent Jobs

Default: 1 worker. For multi-GPU setups:

```bash
export DIFFBOX_NUM_WORKERS=2
supervisorctl restart diffbox
```

## Updating

To update the deployment:

```bash
# Rebuild with latest code
docker build -f Dockerfile.runpod -t your-username/diffbox-runpod:latest .
docker push your-username/diffbox-runpod:latest

# On RunPod: stop pod, change image, restart
# Your models in /workspace/models will persist
```

## Cost Estimation

**Typical RunPod Costs (RTX 4090):**
- **Idle**: $0.34/hour
- **Running inference**: $0.44/hour
- **Model download phase**: $0.44/hour

**First deployment**: ~1-2 hours for model downloads (~$0.88)
**Subsequent restarts**: Instant (models cached)

## Backup Strategy

**Critical data to back up:**
```bash
/workspace/models/    # ~100GB - model weights (redownloadable)
/workspace/data/      # <1MB - SQLite database (job history)
/workspace/outputs/   # Variable - generated videos/images
```

Only `/data` and `/outputs` need regular backups. Models can be re-downloaded.

## Security Considerations

**Default deployment is PUBLIC**. To secure:

1. **Enable authentication** in diffbox (TODO: implement auth)
2. **Use RunPod's private network** (for multi-pod setups)
3. **Restrict IP access** via RunPod firewall settings
4. **Don't expose port 8188** publicly (ComfyUI has no auth)

## Support

**Logs location:**
- diffbox: `/var/log/diffbox/`
- ComfyUI: `/var/log/comfyui/`
- supervisor: `/var/log/supervisor/`

**Health check:**
```bash
/usr/local/bin/healthcheck.sh
```

**Restart services:**
```bash
supervisorctl restart comfyui
supervisorctl restart diffbox
supervisorctl restart all
```
