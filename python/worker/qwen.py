"""
Qwen Image Edit 2511 handler with real diffsynth inference.
"""

import sys
from pathlib import Path
from typing import Optional
import base64
from io import BytesIO

import torch
from PIL import Image

from worker.protocol import send_progress

# Import diffsynth components
from diffsynth.pipelines.qwen_image import QwenImagePipeline
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


class QwenHandler:
    """Handler for Qwen Image Edit 2511 workflow."""

    def __init__(self, models_dir: str, outputs_dir: str):
        self.models_dir = Path(models_dir)
        self.outputs_dir = Path(outputs_dir)
        self.pipeline: Optional[QwenImagePipeline] = None

    def _load_pipeline(self):
        """Lazy load the Qwen Image Edit pipeline."""
        if self.pipeline is not None:
            return

        print("Loading Qwen Image Edit 2511 pipeline...", file=sys.stderr)
        send_progress(None, 0.0, "Loading Qwen pipeline...")

        # Model paths (downloaded by aria2 on startup)
        dit_path = self.models_dir / "qwen_image_edit_2511_bf16.safetensors"
        text_encoder_path = self.models_dir / "qwen_2.5_vl_7b.safetensors"
        vae_path = self.models_dir / "qwen_image_vae.safetensors"

        # Validate models exist
        for path in [dit_path, text_encoder_path, vae_path]:
            if not path.exists():
                raise FileNotFoundError(f"Required model not found: {path}")

        # Create model configs
        model_configs = [
            ModelConfig(path=str(dit_path)),
            ModelConfig(path=str(text_encoder_path)),
            ModelConfig(path=str(vae_path)),
        ]

        # Initialize pipeline
        # Note: tokenizer/processor are loaded from HF for Qwen2VL
        self.pipeline = QwenImagePipeline.from_pretrained(
            torch_dtype=torch.bfloat16,
            device="cuda",
            model_configs=model_configs,
            # Use Qwen2VL processor for image editing
            processor_config=ModelConfig(
                model_id="Qwen/Qwen2-VL-7B-Instruct",
                download_source="huggingface",
            ),
        )

        print("Pipeline loaded.", file=sys.stderr)
        send_progress(None, 0.0, "Pipeline loaded")

    def run(self, job_id: str, params: dict) -> dict:
        """Run Qwen image edit inference."""
        self._load_pipeline()

        # Extract parameters with defaults
        instruction = params.get("instruction", params.get("prompt", ""))
        edit_images_b64 = params.get("edit_images", [])
        seed = params.get("seed")
        height = params.get("height", 1024)
        width = params.get("width", 1024)
        num_inference_steps = params.get("num_inference_steps", 30)
        cfg_scale = params.get("cfg_scale", 4.0)

        # Decode input images from base64 (up to 3)
        edit_images = []
        for i, img_b64 in enumerate(edit_images_b64[:3]):
            if img_b64:
                send_progress(job_id, 0.01 + (i * 0.01), f"Decoding image {i + 1}")
                image_data = base64.b64decode(img_b64)
                img = Image.open(BytesIO(image_data)).convert("RGB")
                edit_images.append(img)

        if not edit_images:
            raise ValueError("At least one edit_image is required for Qwen Image Edit")

        # Set random seed
        if seed is None or seed == -1:
            seed = torch.randint(0, 2**32 - 1, (1,)).item()

        send_progress(job_id, 0.05, "Starting inference")

        # Create progress tracker
        progress_tracker = ProgressTracker(job_id, num_inference_steps)

        # Prepare edit_image parameter
        # Single image -> Image.Image, multiple images -> list[Image.Image]
        edit_image_param = edit_images[0] if len(edit_images) == 1 else edit_images

        # Run inference
        output_image = self.pipeline(
            prompt=instruction,
            edit_image=edit_image_param,
            height=height,
            width=width,
            num_inference_steps=num_inference_steps,
            cfg_scale=cfg_scale,
            seed=seed,
            edit_image_auto_resize=True,
            progress_bar_cmd=progress_tracker,
        )

        send_progress(job_id, 0.95, "Saving image")

        # Convert output if needed
        if isinstance(output_image, torch.Tensor):
            # Convert tensor to PIL Image
            output_np = output_image.cpu().numpy()
            if output_np.max() <= 1.0:
                output_np = (output_np * 255).astype("uint8")
            else:
                output_np = output_np.astype("uint8")
            output_image = Image.fromarray(output_np)

        # Save output
        self.outputs_dir.mkdir(parents=True, exist_ok=True)
        output_path = self.outputs_dir / f"{job_id}.png"
        output_image.save(output_path)

        send_progress(job_id, 1.0, "Complete")

        return {
            "type": "image",
            "path": str(output_path),
            "seed": seed,
        }
