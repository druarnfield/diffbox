"""
Qwen Image Edit 2511 handler with real diffsynth inference.
"""

import base64
import logging
import time
from io import BytesIO
from pathlib import Path
from typing import Optional

import torch

# Import diffsynth components - use new API with ModelConfig
from diffsynth.pipelines.qwen_image import QwenImagePipeline
from diffsynth.utils import ModelConfig
from PIL import Image

from worker.protocol import send_progress

logger = logging.getLogger("worker.qwen")


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
                f"Denoising step {self.current_step}/{self.total_steps}",
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

        logger.info("Loading Qwen Image Edit 2511 pipeline...")
        send_progress(None, 0.0, "Loading Qwen pipeline...")

        # Log GPU info
        if torch.cuda.is_available():
            gpu_name = torch.cuda.get_device_name(0)
            total_memory = torch.cuda.get_device_properties(0).total_memory / 1e9
            logger.info(f"GPU: {gpu_name}")
            logger.info(f"Total VRAM: {total_memory:.1f} GB")
        else:
            logger.warning("CUDA not available - will run on CPU (very slow!)")

        # Model paths (downloaded by aria2 on startup)
        dit_path = self.models_dir / "qwen_image_edit_2511_bf16.safetensors"
        text_encoder_path = self.models_dir / "qwen_2.5_vl_7b.safetensors"
        vae_path = self.models_dir / "qwen_image_vae.safetensors"
        lightning_lora_path = (
            self.models_dir
            / "Qwen-Image-Edit-2511-Lightning-4steps-V1.0-bf16.safetensors"
        )

        # Validate models exist
        for path in [dit_path, text_encoder_path, vae_path]:
            if not path.exists():
                error_msg = f"Required model not found: {path}"
                logger.error(error_msg)
                raise FileNotFoundError(error_msg)

        logger.info("All model files found, loading pipeline with from_pretrained...")

        # Use new DiffSynth API with ModelConfig for local files
        model_configs = [
            ModelConfig(path=str(dit_path)),
            ModelConfig(path=str(text_encoder_path)),
            ModelConfig(path=str(vae_path)),
        ]

        # Add Lightning LoRA if available
        if lightning_lora_path.exists():
            logger.info("Including Lightning LoRA for 4-step inference...")
            model_configs.append(ModelConfig(path=str(lightning_lora_path)))
        else:
            logger.warning(
                f"Lightning LoRA not found at {lightning_lora_path}, using standard inference"
            )

        # Load pipeline with from_pretrained using local model paths
        # Use HuggingFace for tokenizer instead of ModelScope (avoids auth issues)
        tokenizer_config = ModelConfig(
            model_id="Qwen/Qwen2.5-VL-7B-Instruct",
            origin_file_pattern="",  # Download tokenizer files
            download_resource="huggingface",
        )

        self.pipeline = QwenImagePipeline.from_pretrained(
            torch_dtype=torch.bfloat16,
            device="cuda",
            model_configs=model_configs,
            tokenizer_config=tokenizer_config,
        )

        # Log VRAM usage after loading
        if torch.cuda.is_available():
            allocated = torch.cuda.memory_allocated(0) / 1e9
            reserved = torch.cuda.memory_reserved(0) / 1e9
            logger.info(
                f"Pipeline loaded - VRAM allocated: {allocated:.1f} GB, reserved: {reserved:.1f} GB"
            )

        send_progress(None, 0.0, "Pipeline loaded")

    def run(self, job_id: str, params: dict) -> dict:
        """Run Qwen image edit inference."""
        start_time = time.time()

        logger.info(f"Starting Qwen job {job_id}")
        # Log params without image data (which can be megabytes of base64)
        safe_params = {
            k: (f"<{len(v)} chars>" if k == "edit_images" else v)
            for k, v in params.items()
        }
        logger.info(f"Parameters: {safe_params}")

        self._load_pipeline()

        # Extract parameters with defaults (optimized for Lightning LoRA)
        instruction = params.get("instruction", params.get("prompt", ""))
        edit_images_b64 = params.get("edit_images", [])
        seed = params.get("seed")
        height = params.get("height", 1024)
        width = params.get("width", 1024)
        num_inference_steps = params.get(
            "num_inference_steps", 4
        )  # Lightning LoRA uses 4 steps
        cfg_scale = params.get("cfg_scale", 1.0)  # Lightning LoRA uses minimal CFG

        # Decode input images from base64 (up to 3)
        edit_images = []
        for i, img_b64 in enumerate(edit_images_b64[:3]):
            if img_b64:
                send_progress(job_id, 0.01 + (i * 0.01), f"Decoding image {i + 1}")
                image_data = base64.b64decode(img_b64)
                img = Image.open(BytesIO(image_data)).convert("RGB")
                edit_images.append(img)
                logger.info(f"Input image {i + 1} size: {img.size}")

        if not edit_images:
            raise ValueError("At least one edit_image is required for Qwen Image Edit")

        # Set random seed
        if seed is None or seed == -1:
            seed = torch.randint(0, 2**32 - 1, (1,)).item()
        logger.info(f"Using seed: {seed}")

        # Log VRAM before inference
        if torch.cuda.is_available():
            free_memory = (
                torch.cuda.get_device_properties(0).total_memory
                - torch.cuda.memory_allocated(0)
            ) / 1e9
            logger.info(f"Free VRAM before inference: {free_memory:.1f} GB")

        send_progress(job_id, 0.05, "Starting inference")

        # Create progress tracker
        progress_tracker = ProgressTracker(job_id, num_inference_steps)

        # Prepare edit_image parameter
        # Single image -> Image.Image, multiple images -> list[Image.Image]
        edit_image_param = edit_images[0] if len(edit_images) == 1 else edit_images

        inference_start = time.time()

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

        inference_duration = time.time() - inference_start
        logger.info(f"Inference completed in {inference_duration:.1f}s")

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

        total_duration = time.time() - start_time
        logger.info(f"Job {job_id} total time: {total_duration:.1f}s")
        logger.info(f"Image saved to {output_path}")

        send_progress(job_id, 1.0, "Complete")

        return {
            "type": "image",
            "path": str(output_path),
            "seed": seed,
        }
