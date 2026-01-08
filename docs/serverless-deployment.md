# RunPod Serverless Deployment Guide

This guide explains how to deploy diffbox using RunPod Serverless for cost-effective GPU inference.

## Architecture

**Serverless Architecture (Recommended):**
```
┌─────────────────┐         HTTP          ┌──────────────────────┐
│  diffbox server │ ────────────────────> │ RunPod Serverless    │
│  (local/VPS)    │                       │ (GPU workers)        │
│                 │ <──────────────────── │                      │
│  - Go API       │       Response        │ - ComfyUI            │
│  - Frontend     │                       │ - Dolphin-Mistral    │
│  - Job tracking │                       │ - Inference handlers │
└─────────────────┘                       └──────────────────────┘
   No GPU needed                              Auto-scaling
   Always running                             Pay-per-use
```

**Benefits:**
- **Cost-effective:** Only pay for actual inference time (per second)
- **Auto-scaling:** RunPod handles scaling workers up/down
- **No GPU needed locally:** Run web server on cheap VPS or local machine
- **Faster cold starts:** RunPod keeps models cached

## Prerequisites

1. **RunPod Account:** Sign up at https://runpod.io
2. **Docker Hub Account:** For pushing your container image
3. **Local machine:** For running the diffbox web server

## Step 1: Build and Push Serverless Image

```bash
# Build the serverless image
make docker-serverless

# Push to Docker Hub (replace 'your-username' with your Docker Hub username)
make push-serverless REGISTRY=your-username
```

This creates an image containing:
- ComfyUI server
- Python inference workers (I2V, Qwen, Chat)
- RunPod handler
- All dependencies

## Step 2: Create RunPod Serverless Endpoint

1. Go to https://runpod.io/console/serverless
2. Click **"New Endpoint"**
3. Configure:
   - **Name:** diffbox-inference
   - **Container Image:** `your-username/diffbox-serverless:latest`
   - **GPU Type:** RTX 4090 or A40 (24GB+ VRAM required)
   - **Workers:**
     - Min: 0 (scales to zero when idle)
     - Max: 3 (adjust based on load)
   - **Container Disk:** 50GB
   - **Network Volume:** Create new 100GB volume
     - Mount path: `/runpod-volume`
     - This persists models between worker restarts

4. Click **"Deploy"**
5. Copy the **Endpoint ID** (you'll need this)

## Step 3: Configure Local Server

Set the RunPod endpoint environment variable:

```bash
# Export environment variable
export RUNPOD_ENDPOINT_ID=your-endpoint-id
export RUNPOD_API_KEY=your-api-key  # Get from RunPod account settings

# Or add to .env file
echo "RUNPOD_ENDPOINT_ID=your-endpoint-id" >> .env
echo "RUNPOD_API_KEY=your-api-key" >> .env
```

## Step 4: Run Local Server

```bash
# Build and run the web server (no GPU needed)
make build
./diffbox
```

The server will:
- Run on http://localhost:8080
- Forward inference jobs to RunPod Serverless
- Track job status and handle WebSocket updates
- Serve the frontend

## Step 5: First Run - Model Downloads

On first run, RunPod workers will download models to the network volume:
- Dolphin-Mistral: ~49GB (10 shards)
- ComfyUI models: varies by workflow

**Monitor downloads:**
- Check RunPod logs in the serverless dashboard
- Models persist on the network volume
- Subsequent runs use cached models

## Cost Estimation

**Example costs (RunPod RTX 4090):**
- Idle: $0/hour (scales to zero)
- Active inference: ~$0.50/hour (only charged during execution)
- Network volume: ~$5/month (100GB)

**vs. On-Demand Pod:**
- 24/7 running: ~$12-24/day
- With serverless: Only pay for actual usage

## Monitoring

**Check endpoint status:**
```bash
curl -X GET https://api.runpod.ai/v2/{endpoint_id}/status \
  -H "Authorization: Bearer ${RUNPOD_API_KEY}"
```

**View logs:**
- Go to RunPod console → Serverless → Your Endpoint → Logs
- Filter by worker ID to see specific inference runs

## Troubleshooting

### Cold starts are slow
- First request after idle: 30-60 seconds (loading models)
- Subsequent requests: 1-5 seconds
- Solution: Keep 1 min worker running during peak hours

### Models not persisting
- Ensure network volume is mounted to `/runpod-volume`
- Check volume permissions
- Verify models downloaded to `/runpod-volume/models`

### Worker timeouts
- Increase execution timeout in endpoint settings
- Default: 300s, increase to 600s for large jobs
- Monitor GPU memory usage

### Out of VRAM errors
- Dolphin-Mistral needs 24GB+ VRAM
- Use RTX 4090 (24GB) or A40 (48GB)
- Don't use RTX 3090 (24GB is too tight)

## Scaling

**Auto-scaling configuration:**
- **Workers:** Start with 0 min, 3 max
- **Scale-up:** Immediate when jobs queued
- **Scale-down:** After 5 minutes idle
- **Adjust based on usage patterns**

**High-traffic setup:**
- Set min workers to 1-2 to avoid cold starts
- Increase max workers to 5-10
- Monitor queue depth and adjust

## Development Workflow

**Local development:**
```bash
# Run server locally (forwards to serverless)
make run

# Or with hot reload
make dev
```

**Testing serverless handler locally:**
```bash
# Requires GPU
docker run --gpus all -p 8000:8000 \
  -v $(pwd)/models:/runpod-volume/models \
  your-username/diffbox-serverless:latest
```

## Alternative: Hybrid Deployment

Run some workflows locally, others on serverless:

```bash
# Set per-workflow endpoints
export I2V_ENDPOINT_ID=endpoint-for-video
export CHAT_ENDPOINT_ID=endpoint-for-chat
export QWEN_ENDPOINT_ID=endpoint-for-images
```

This allows:
- Heavy workflows (I2V) → Serverless
- Fast workflows (Chat) → Local GPU
- Mix and match based on cost/performance

## Next Steps

1. Review `runpod_handler.py` for customization
2. Add custom workflows to ComfyUI
3. Update `runpod-template.json` with your settings
4. Monitor costs in RunPod dashboard
5. Optimize worker scaling based on usage

## Resources

- RunPod Serverless Docs: https://docs.runpod.io/serverless/overview
- RunPod API Reference: https://docs.runpod.io/api/reference
- Cost Calculator: https://runpod.io/console/serverless/pricing
