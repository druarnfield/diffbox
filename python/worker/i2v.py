"""
Wan 2.2 Image-to-Video handler with real diffsynth inference.
"""

import os
import sys
import subprocess
import tempfile
from pathlib import Path
from typing import Optional
import base64
from io import BytesIO

import torch
from PIL import Image

from worker.protocol import send_progress

# Import diffsynth components
from diffsynth.pipelines.wan_video import WanVideoPipeline
from diffsynth.core import ModelConfig


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

    def _load_pipeline(self):
        """Lazy load the Wan 2.2 I2V pipeline."""
        if self.pipeline is not None:
            return

        print("Loading Wan 2.2 I2V pipeline...", file=sys.stderr)
        send_progress(None, 0.0, "Loading I2V pipeline...")

        # Model paths (downloaded by aria2 on startup)
        high_noise_path = self.models_dir / "wan2.2_i2v_high_noise_14B_fp16.safetensors"
        low_noise_path = self.models_dir / "wan2.2_i2v_low_noise_14B_fp16.safetensors"
        text_encoder_path = self.models_dir / "umt5_xxl_fp16.safetensors"
        vae_path = self.models_dir / "wan_2.1_vae.safetensors"

        # Validate models exist
        for path in [high_noise_path, low_noise_path, text_encoder_path, vae_path]:
            if not path.exists():
                raise FileNotFoundError(f"Required model not found: {path}")

        # Create model configs
        model_configs = [
            ModelConfig(path=str(high_noise_path)),
            ModelConfig(path=str(low_noise_path)),
            ModelConfig(path=str(text_encoder_path)),
            ModelConfig(path=str(vae_path)),
        ]

        # Initialize pipeline
        self.pipeline = WanVideoPipeline.from_pretrained(
            torch_dtype=torch.bfloat16,
            device="cuda",
            model_configs=model_configs,
        )

        print("Pipeline loaded.", file=sys.stderr)
        send_progress(None, 0.0, "Pipeline loaded")

    def run(self, job_id: str, params: dict) -> dict:
        """Run I2V inference."""
        self._load_pipeline()

        # Extract parameters with defaults
        prompt = params.get("prompt", "")
        negative_prompt = params.get("negative_prompt", "")
        input_image_b64 = params.get("input_image")
        seed = params.get("seed")
        height = params.get("height", 480)
        width = params.get("width", 832)
        num_frames = params.get("num_frames", 81)
        num_inference_steps = params.get("num_inference_steps", 50)
        cfg_scale = params.get("cfg_scale", 5.0)
        denoising_strength = params.get("denoising_strength", 1.0)
        tiled = params.get("tiled", True)
        tile_size = params.get("tile_size", (30, 52))

        # Decode input image from base64
        if not input_image_b64:
            raise ValueError("input_image is required for I2V")

        send_progress(job_id, 0.02, "Decoding input image")
        image_data = base64.b64decode(input_image_b64)
        input_image = Image.open(BytesIO(image_data)).convert("RGB")

        # Set random seed
        if seed is None or seed == -1:
            seed = torch.randint(0, 2**32 - 1, (1,)).item()

        send_progress(job_id, 0.05, "Starting inference")

        # Create progress tracker
        progress_tracker = ProgressTracker(job_id, num_inference_steps)

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

        send_progress(job_id, 0.95, "Encoding video")

        # Save video
        self.outputs_dir.mkdir(parents=True, exist_ok=True)
        output_path = self.outputs_dir / f"{job_id}.mp4"
        self._save_video(video_frames, output_path)

        send_progress(job_id, 1.0, "Complete")

        return {
            "type": "video",
            "path": str(output_path),
            "frames": num_frames,
            "seed": seed,
        }

    def _save_video(self, frames: list, output_path: Path, fps: int = 24):
        """Save frames as MP4 video using ffmpeg."""
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
                check=True,
            )
            print(f"Video saved to {output_path}", file=sys.stderr)
