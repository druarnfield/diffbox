"""
RunPod Serverless handler for diffbox inference workloads.

This handler receives HTTP requests from the diffbox server and routes them
to the appropriate inference handler (I2V, Qwen, Chat).

Supports RunPod warm-start feature by pre-loading models on container start.
"""

import runpod
import base64
import logging
import os
from pathlib import Path
from io import BytesIO
from PIL import Image

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Lazy-load handlers to avoid loading models until needed
_handlers = {}

# Pre-warm flag - load models on container start for faster first request
PRELOAD_MODELS = os.getenv("PRELOAD_MODELS", "true").lower() == "true"
PRELOAD_TYPES = os.getenv("PRELOAD_TYPES", "chat").split(",")  # Default: only chat


def get_handler(job_type: str):
    """Get or create handler for job type."""
    if job_type not in _handlers:
        models_dir = os.getenv("DIFFBOX_MODELS_DIR", "/runpod-volume/models")
        outputs_dir = os.getenv("DIFFBOX_OUTPUTS_DIR", "/runpod-volume/outputs")

        if job_type == "i2v":
            from worker.i2v import I2VHandler
            _handlers[job_type] = I2VHandler(models_dir, outputs_dir)
        elif job_type == "qwen":
            from worker.qwen import QwenHandler
            _handlers[job_type] = QwenHandler(models_dir, outputs_dir)
        elif job_type == "chat":
            from worker.chat import ChatHandler
            _handlers[job_type] = ChatHandler(models_dir, outputs_dir)
        else:
            raise ValueError(f"Unknown job type: {job_type}")

    return _handlers[job_type]


def handler(event):
    """
    RunPod serverless handler.

    Expected input:
    {
        "input": {
            "type": "i2v" | "qwen" | "chat",
            "params": { ... }  // job-specific parameters
        }
    }

    Returns:
    {
        "output": {
            "type": "video" | "image" | "text",
            "path": "...",  // for video/image
            "response": "...",  // for chat
            "data": "...",  // base64 encoded file data
            ...
        }
    }
    """
    try:
        input_data = event.get("input", {})
        job_type = input_data.get("type")
        params = input_data.get("params", {})

        logger.info(f"Processing {job_type} job")

        # Get handler and run inference
        handler = get_handler(job_type)
        job_id = event.get("id", "unknown")
        result = handler.run(job_id, params)

        # For video/image results, encode the output file as base64
        if "path" in result and result["type"] in ["video", "image"]:
            output_path = result["path"]
            if os.path.exists(output_path):
                with open(output_path, "rb") as f:
                    file_data = f.read()
                result["data"] = base64.b64encode(file_data).decode("utf-8")
                # Remove path from response (not needed by client)
                del result["path"]
            else:
                logger.warning(f"Output file not found: {output_path}")

        return result

    except Exception as e:
        logger.error(f"Handler error: {e}", exc_info=True)
        return {
            "error": str(e),
            "error_type": type(e).__name__
        }


def preload_models():
    """Pre-load models on container start for warm-start optimization."""
    if not PRELOAD_MODELS:
        logger.info("Model preloading disabled (PRELOAD_MODELS=false)")
        return

    logger.info(f"Pre-loading models: {PRELOAD_TYPES}")
    for job_type in PRELOAD_TYPES:
        job_type = job_type.strip()
        if job_type:
            try:
                logger.info(f"Loading {job_type} model...")
                get_handler(job_type)
                logger.info(f"✓ {job_type} model loaded")
            except Exception as e:
                logger.warning(f"Failed to preload {job_type}: {e}")

    logger.info("Model preloading complete - worker ready for requests")


# Start RunPod serverless worker
if __name__ == "__main__":
    # Pre-load models for warm starts
    preload_models()

    # Start serverless handler
    runpod.serverless.start({"handler": handler})
