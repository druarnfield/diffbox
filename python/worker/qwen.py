"""
Qwen Image Edit 2511 handler using ComfyUI backend.
"""

import asyncio
import base64
import logging
import os
import time
from io import BytesIO
from pathlib import Path
from typing import Optional

from PIL import Image

from worker.comfyui_client import ComfyUIClient
from worker.comfyui_templates import ComfyUIWorkflowBuilder
from worker.protocol import send_progress

logger = logging.getLogger("worker.qwen")


class QwenHandler:
    """Handler for Qwen Image Edit 2511 workflow via ComfyUI."""

    def __init__(self, models_dir: str, outputs_dir: str):
        self.models_dir = Path(models_dir)
        self.outputs_dir = Path(outputs_dir)
        self.comfyui_url = os.getenv("COMFYUI_URL", "http://localhost:8188")
        self.client: Optional[ComfyUIClient] = None
        self.workflow_builder: Optional[ComfyUIWorkflowBuilder] = None

    def _init_client(self):
        """Initialize ComfyUI client and workflow builder (lazy)."""
        if self.client is None:
            logger.info(f"Initializing ComfyUI client: {self.comfyui_url}")
            self.client = ComfyUIClient(base_url=self.comfyui_url)
            self.workflow_builder = ComfyUIWorkflowBuilder()
            send_progress(None, 0.0, "ComfyUI client initialized")

    def run(self, job_id: str, params: dict) -> dict:
        """Run Qwen image edit inference via ComfyUI."""
        start_time = time.time()

        logger.info(f"Starting Qwen job {job_id}")
        # Log params without image data
        safe_params = {
            k: (f"<{len(v)} chars>" if k == "edit_images" else v)
            for k, v in params.items()
        }
        logger.info(f"Parameters: {safe_params}")

        self._init_client()

        # Extract parameters
        instruction = params.get("instruction", params.get("prompt", ""))
        edit_images_b64 = params.get("edit_images", [])
        seed = params.get("seed")
        cfg_scale = params.get("cfg_scale", 7.0)
        steps = params.get("num_inference_steps", 28)

        # Decode and upload first input image
        # (Qwen supports up to 3 images, but for now we'll use the first one)
        if not edit_images_b64:
            raise ValueError("At least one edit_image is required for Qwen Image Edit")

        send_progress(job_id, 0.05, "Uploading input image")
        image_data = base64.b64decode(edit_images_b64[0])
        input_image = Image.open(BytesIO(image_data)).convert("RGB")
        logger.info(f"Input image size: {input_image.size}")

        # Generate unique filename for upload
        upload_filename = f"qwen_input_{job_id}.png"

        # Upload image to ComfyUI
        uploaded_filename = asyncio.run(
            self.client.upload_image(input_image, upload_filename)
        )
        logger.info(f"Image uploaded as: {uploaded_filename}")

        # Handle seed
        if seed is None or seed == -1:
            import random

            seed = random.randint(0, 2**32 - 1)
        logger.info(f"Using seed: {seed}")

        # Build workflow from template
        send_progress(job_id, 0.10, "Building workflow")
        workflow = self.workflow_builder.build_qwen(
            instruction=instruction,
            image_path=uploaded_filename,
            mask_path=None,  # TODO: Support inpainting mask
            seed=seed,
            cfg_scale=cfg_scale,
            steps=steps,
        )

        # Validate workflow
        self.workflow_builder.validate_workflow(workflow)
        logger.info(f"Workflow built with {len(workflow)} nodes")

        # Progress callback for ComfyUI execution
        def on_progress(progress: float, stage: str):
            # Map 0.0-1.0 to 10%-95% range
            mapped_progress = 0.10 + (progress * 0.85)
            send_progress(job_id, mapped_progress, stage)

        # Execute workflow
        send_progress(job_id, 0.10, "Starting ComfyUI execution")
        inference_start = time.time()

        result = asyncio.run(
            self.client.execute_workflow(
                workflow=workflow, on_progress=on_progress, timeout=600
            )
        )

        inference_duration = time.time() - inference_start
        logger.info(f"Inference completed in {inference_duration:.1f}s")

        # Extract output file info
        outputs = result["outputs"]
        if "image" not in outputs:
            raise RuntimeError("No image output found in ComfyUI result")

        image_info = outputs["image"]
        logger.info(f"Output image: {image_info}")

        # Download output image
        send_progress(job_id, 0.95, "Downloading output image")
        image_bytes = asyncio.run(
            self.client.download_output(
                filename=image_info["filename"],
                subfolder=image_info.get("subfolder", ""),
                output_type=image_info.get("type", "output"),
            )
        )

        # Save to outputs directory
        self.outputs_dir.mkdir(parents=True, exist_ok=True)
        output_path = self.outputs_dir / f"{job_id}.png"

        with open(output_path, "wb") as f:
            f.write(image_bytes)

        logger.info(f"Image saved to {output_path}")

        total_duration = time.time() - start_time
        logger.info(f"Job {job_id} total time: {total_duration:.1f}s")

        send_progress(job_id, 1.0, "Complete")

        return {
            "type": "image",
            "path": str(output_path),
            "seed": seed,
        }
