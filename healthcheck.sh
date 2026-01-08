#!/bin/bash
# Health check script for RunPod deployment

set -e

# Check if ComfyUI is responding
if ! curl -sf http://localhost:8188 > /dev/null; then
    echo "ComfyUI not responding on port 8188"
    exit 1
fi

# Check if diffbox is responding
if ! curl -sf http://localhost:8080/api/health > /dev/null; then
    echo "diffbox not responding on port 8080"
    exit 1
fi

echo "All services healthy"
exit 0
