"""
Wan 2.2 Image-to-Video handler with real diffsynth inference.
"""

import subprocess
import tempfile
import time
import logging
from pathlib import Path
from typing import Optional
import base64
from io import BytesIO

import torch
from PIL import Image

from worker.protocol import send_progress

# Import diffsynth components
from diffsynth import WanVideoPipeline, ModelManager

logger = logging.getLogger('worker.i2v')


class ProgressTracker:
    """Custom progress tracker that sends updates via protocol."""

    def __init__(self, job_id: str, total_steps: int):
        self.job_id = job_id
        self.total_steps = total_steps
        self.current_step = 0

    def __call__(self, iterable):
        """Wrap iterable to track progress."""
        for item in iterable:
            yield item
            self.current_step += 1
            # Map to 5-95% range (leaving room for load and encode)
            progress = 0.05 + (self.current_step / self.total_steps) * 0.90
            send_progress(
                self.job_id,
                progress,
                f"Denoising step {self.current_step}/{self.total_steps}"
            )


class I2VHandler:
    """Handler for Wan 2.2 I2V (Image-to-Video) workflow."""

    def __init__(self, models_dir: str, outputs_dir: str):
        self.models_dir = Path(models_dir)
        self.outputs_dir = Path(outputs_dir)
        self.pipeline: Optional[WanVideoPipeline] = None
        self.model_manager: Optional[ModelManager] = None

    def _load_pipeline(self):
        """Lazy load the Wan 2.2 I2V pipeline."""
        if self.pipeline is not None:
            return

        logger.info("Loading Wan 2.2 I2V pipeline...")
        send_progress(None, 0.0, "Loading I2V pipeline...")

        # Log GPU info
        if torch.cuda.is_available():
            gpu_name = torch.cuda.get_device_name(0)
            total_memory = torch.cuda.get_device_properties(0).total_memory / 1e9
            logger.info(f"GPU: {gpu_name}")
            logger.info(f"Total VRAM: {total_memory:.1f} GB")
        else:
            logger.warning("CUDA not available - will run on CPU (very slow!)")

        # Model paths (downloaded by aria2 on startup)
        high_noise_path = self.models_dir / "wan2.2_i2v_high_noise_14B_fp16.safetensors"
        low_noise_path = self.models_dir / "wan2.2_i2v_low_noise_14B_fp16.safetensors"
        text_encoder_path = self.models_dir / "umt5_xxl_fp16.safetensors"
        vae_path = self.models_dir / "wan_2.1_vae.safetensors"
        lightning_high_noise_path = self.models_dir / "wan2.2_lightning_high_noise.safetensors"
        lightning_low_noise_path = self.models_dir / "wan2.2_lightning_low_noise.safetensors"

        # Validate models exist
        for path in [high_noise_path, low_noise_path, text_encoder_path, vae_path]:
            if not path.exists():
                error_msg = f"Required model not found: {path}"
                logger.error(error_msg)
                raise FileNotFoundError(error_msg)

        logger.info("All model files found, initializing ModelManager...")

        # Initialize ModelManager with file paths
        file_paths = [
            str(high_noise_path),
            str(low_noise_path),
            str(text_encoder_path),
            str(vae_path),
        ]

        self.model_manager = ModelManager(
            torch_dtype=torch.bfloat16,
            device="cuda",
            file_path_list=file_paths
        )

        # Initialize pipeline
        self.pipeline = WanVideoPipeline(device="cuda", torch_dtype=torch.bfloat16)

        # Load models into pipeline
        logger.info("Loading models into pipeline...")
        self.model_manager.load_models(file_paths)

        # Fetch and assign models to pipeline
        self.pipeline.text_encoder = self.model_manager.fetch_model("text_encoder")
        self.pipeline.dit = self.model_manager.fetch_model("dit")
        self.pipeline.vae = self.model_manager.fetch_model("vae")

        # Load Lightning LoRAs for 4-step inference if available
        if lightning_high_noise_path.exists() and lightning_low_noise_path.exists():
            logger.info("Loading Lightning LoRAs for 4-step inference...")
            self.model_manager.load_lora(file_path=str(lightning_high_noise_path), lora_alpha=1.0)
            self.model_manager.load_lora(file_path=str(lightning_low_noise_path), lora_alpha=1.0)
            logger.info("Lightning LoRAs loaded")
        else:
            logger.warning("Lightning LoRAs not found, using standard 50-step inference")

        # Log VRAM usage after loading
        if torch.cuda.is_available():
            allocated = torch.cuda.memory_allocated(0) / 1e9
            reserved = torch.cuda.memory_reserved(0) / 1e9
            logger.info(f"Pipeline loaded - VRAM allocated: {allocated:.1f} GB, reserved: {reserved:.1f} GB")

        send_progress(None, 0.0, "Pipeline loaded")

    def run(self, job_id: str, params: dict) -> dict:
        """Run I2V inference."""
        start_time = time.time()

        logger.info(f"Starting I2V job {job_id}")
        logger.info(f"Parameters: {params}")

        self._load_pipeline()

        # Extract parameters with defaults (optimized for Lightning LoRA)
        prompt = params.get("prompt", "")
        negative_prompt = params.get("negative_prompt", "")
        input_image_b64 = params.get("input_image")
        seed = params.get("seed")
        height = params.get("height", 480)
        width = params.get("width", 832)
        num_frames = params.get("num_frames", 81)
        num_inference_steps = params.get("num_inference_steps", 4)  # Lightning LoRA uses 4 steps
        cfg_scale = params.get("cfg_scale", 1.0)  # Lightning LoRA uses minimal CFG
        denoising_strength = params.get("denoising_strength", 1.0)
        tiled = params.get("tiled", True)
        tile_size = params.get("tile_size", (30, 52))

        # Decode input image from base64
        if not input_image_b64:
            raise ValueError("input_image is required for I2V")

        send_progress(job_id, 0.02, "Decoding input image")
        image_data = base64.b64decode(input_image_b64)
        input_image = Image.open(BytesIO(image_data)).convert("RGB")
        logger.info(f"Input image size: {input_image.size}")

        # Set random seed
        if seed is None or seed == -1:
            seed = torch.randint(0, 2**32 - 1, (1,)).item()
        logger.info(f"Using seed: {seed}")

        # Log VRAM before inference
        if torch.cuda.is_available():
            free_memory = (torch.cuda.get_device_properties(0).total_memory -
                          torch.cuda.memory_allocated(0)) / 1e9
            logger.info(f"Free VRAM before inference: {free_memory:.1f} GB")

        send_progress(job_id, 0.05, "Starting inference")

        # Create progress tracker
        progress_tracker = ProgressTracker(job_id, num_inference_steps)

        inference_start = time.time()

        # Run inference
        video_frames = self.pipeline(
            prompt=prompt,
            negative_prompt=negative_prompt,
            input_image=input_image,
            height=height,
            width=width,
            num_frames=num_frames,
            num_inference_steps=num_inference_steps,
            cfg_scale=cfg_scale,
            denoising_strength=denoising_strength,
            seed=seed,
            tiled=tiled,
            tile_size=tuple(tile_size) if isinstance(tile_size, list) else tile_size,
            progress_bar_cmd=progress_tracker,
        )

        inference_duration = time.time() - inference_start
        fps_generation = num_frames / inference_duration
        logger.info(f"Inference completed in {inference_duration:.1f}s ({fps_generation:.2f} frames/sec)")

        send_progress(job_id, 0.95, "Encoding video")

        # Save video
        self.outputs_dir.mkdir(parents=True, exist_ok=True)
        output_path = self.outputs_dir / f"{job_id}.mp4"
        self._save_video(video_frames, output_path)

        total_duration = time.time() - start_time
        logger.info(f"Job {job_id} total time: {total_duration:.1f}s")

        send_progress(job_id, 1.0, "Complete")

        return {
            "type": "video",
            "path": str(output_path),
            "frames": num_frames,
            "seed": seed,
        }

    def _save_video(self, frames: list, output_path: Path, fps: int = 24):
        """Save frames as MP4 video using ffmpeg."""
        encode_start = time.time()
        logger.info(f"Encoding {len(frames)} frames to video at {fps} fps...")

        with tempfile.TemporaryDirectory() as tmpdir:
            # Save frames as images
            for i, frame in enumerate(frames):
                if isinstance(frame, torch.Tensor):
                    # Convert tensor to numpy
                    frame_np = frame.cpu().numpy()
                    # Normalize if needed (assuming 0-1 range)
                    if frame_np.max() <= 1.0:
                        frame_np = (frame_np * 255).astype("uint8")
                    else:
                        frame_np = frame_np.astype("uint8")
                    frame = Image.fromarray(frame_np)
                elif isinstance(frame, Image.Image):
                    pass  # Already a PIL Image
                else:
                    # Try to convert from numpy array
                    import numpy as np
                    if isinstance(frame, np.ndarray):
                        if frame.max() <= 1.0:
                            frame = (frame * 255).astype("uint8")
                        frame = Image.fromarray(frame)

                frame.save(f"{tmpdir}/frame_{i:05d}.png")

            # Encode with ffmpeg
            result = subprocess.run(
                [
                    "ffmpeg", "-y",
                    "-framerate", str(fps),
                    "-i", f"{tmpdir}/frame_%05d.png",
                    "-c:v", "libx264",
                    "-pix_fmt", "yuv420p",
                    "-crf", "18",
                    str(output_path)
                ],
                capture_output=True,
                check=False,
            )

            if result.returncode != 0:
                error_msg = result.stderr.decode('utf-8', errors='replace')
                logger.error(f"ffmpeg encoding failed (exit code {result.returncode})")
                logger.error(f"ffmpeg stderr: {error_msg}")
                raise RuntimeError(f"Video encoding failed: {error_msg[:500]}")  # Truncate long error

            encode_duration = time.time() - encode_start
            logger.info(f"Video encoded in {encode_duration:.1f}s")
            logger.info(f"Video saved to {output_path}")
