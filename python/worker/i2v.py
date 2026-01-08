"""
Wan 2.2 Image-to-Video handler using ComfyUI backend.
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

logger = logging.getLogger("worker.i2v")


class I2VHandler:
    """Handler for Wan 2.2 I2V (Image-to-Video) workflow via ComfyUI."""

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
        """Run I2V inference via ComfyUI."""
        start_time = time.time()

        logger.info(f"Starting I2V job {job_id}")
        # Log params without image data
        safe_params = {
            k: (f"<{len(v)} chars>" if k == "input_image" else v)
            for k, v in params.items()
        }
        logger.info(f"Parameters: {safe_params}")

        self._init_client()

        # Extract parameters
        prompt = params.get("prompt", "")
        input_image_b64 = params.get("input_image")
        seed = params.get("seed")
        num_frames = params.get("num_frames", 49)
        fps = params.get("fps", 8)
        cfg_scale = params.get("cfg_scale", 7.0)
        motion_bucket_id = params.get("motion_bucket_id", 127)

        # Validate input
        if not input_image_b64:
            raise ValueError("input_image is required for I2V")

        # Decode and upload input image
        send_progress(job_id, 0.05, "Uploading input image")
        image_data = base64.b64decode(input_image_b64)
        input_image = Image.open(BytesIO(image_data)).convert("RGB")
        logger.info(f"Input image size: {input_image.size}")

        # Generate unique filename for upload
        upload_filename = f"i2v_input_{job_id}.png"

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
        workflow = self.workflow_builder.build_i2v(
            prompt=prompt,
            image_path=uploaded_filename,
            num_frames=num_frames,
            fps=fps,
            seed=seed,
            cfg_scale=cfg_scale,
            motion_bucket_id=motion_bucket_id,
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
        if "video" not in outputs:
            raise RuntimeError("No video output found in ComfyUI result")

        video_info = outputs["video"]
        logger.info(f"Output video: {video_info}")

        # Download output video
        send_progress(job_id, 0.95, "Downloading output video")
        video_bytes = asyncio.run(
            self.client.download_output(
                filename=video_info["filename"],
                subfolder=video_info.get("subfolder", ""),
                output_type=video_info.get("type", "output"),
            )
        )

        # Save to outputs directory
        self.outputs_dir.mkdir(parents=True, exist_ok=True)
        output_path = self.outputs_dir / f"{job_id}.mp4"

        with open(output_path, "wb") as f:
            f.write(video_bytes)

        logger.info(f"Video saved to {output_path}")

        total_duration = time.time() - start_time
        logger.info(f"Job {job_id} total time: {total_duration:.1f}s")

        send_progress(job_id, 1.0, "Complete")

        return {
            "type": "video",
            "path": str(output_path),
            "frames": num_frames,
            "seed": seed,
        }
