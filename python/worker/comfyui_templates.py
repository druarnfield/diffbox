"""
ComfyUI workflow template system for translating diffbox parameters into workflow JSON.
"""

import json
import logging
from pathlib import Path
from typing import Any, Dict, Optional

from PIL import Image

logger = logging.getLogger(__name__)


class ComfyUIWorkflowBuilder:
    """Builds ComfyUI workflow JSON from diffbox parameters."""

    def __init__(self, templates_dir: Optional[Path] = None):
        """
        Initialize workflow builder.

        Args:
            templates_dir: Directory containing workflow JSON templates
        """
        self.templates_dir = templates_dir or (
            Path(__file__).parent / "comfyui_workflows"
        )
        logger.info(f"ComfyUI templates directory: {self.templates_dir}")

    def _load_template(self, name: str) -> Dict[str, Any]:
        """
        Load a workflow template from disk.

        Args:
            name: Template name (without .json extension)

        Returns:
            Workflow template as dict
        """
        template_path = self.templates_dir / f"{name}.json"

        if not template_path.exists():
            raise FileNotFoundError(
                f"Workflow template not found: {template_path}\n"
                f"Export workflow from ComfyUI and save as {template_path.name}"
            )

        logger.info(f"Loading template: {template_path}")

        with open(template_path) as f:
            return json.load(f)

    def build_i2v(
        self,
        prompt: str,
        image_path: str,
        num_frames: int = 49,
        fps: int = 8,
        seed: Optional[int] = None,
        cfg_scale: float = 7.0,
        motion_bucket_id: int = 127,
    ) -> Dict[str, Any]:
        """
        Build I2V (image-to-video) workflow.

        Args:
            prompt: Text prompt for video generation
            image_path: Path to input image (uploaded to ComfyUI)
            num_frames: Number of frames to generate
            fps: Output video FPS
            seed: Random seed (None = random)
            cfg_scale: Classifier-free guidance scale
            motion_bucket_id: Motion strength (0-255)

        Returns:
            Complete ComfyUI workflow dict
        """
        logger.info(
            f"Building I2V workflow: prompt='{prompt[:50]}...', frames={num_frames}"
        )

        workflow = self._load_template("i2v")

        # Find nodes by class type and update parameters
        for node_id, node in workflow.items():
            class_type = node.get("class_type", "")

            # Load input image
            if class_type == "LoadImage":
                node["inputs"]["image"] = Path(image_path).name

            # Set prompt
            if class_type == "CLIPTextEncode":
                node["inputs"]["text"] = prompt

            # Set sampling parameters
            if class_type == "KSampler":
                node["inputs"]["seed"] = seed if seed is not None else -1
                node["inputs"]["cfg"] = cfg_scale
                node["inputs"]["steps"] = 20  # Default steps

            # Set video parameters
            if class_type == "VideoLinearCFGGuidance":
                node["inputs"]["min_cfg"] = cfg_scale

            # Set frame count
            if class_type in ["SVD_img2vid_Conditioning", "VideoLinearCFGGuidance"]:
                if "frames" in node["inputs"]:
                    node["inputs"]["frames"] = num_frames
                if "motion_bucket_id" in node["inputs"]:
                    node["inputs"]["motion_bucket_id"] = motion_bucket_id

            # Set FPS for output
            if class_type == "VHS_VideoCombine":
                node["inputs"]["frame_rate"] = fps
                node["inputs"]["format"] = "video/h264-mp4"

        logger.debug(f"I2V workflow built with {len(workflow)} nodes")
        return workflow

    def build_qwen(
        self,
        instruction: str,
        image_path: str,
        mask_path: Optional[str] = None,
        seed: Optional[int] = None,
        cfg_scale: float = 7.0,
        steps: int = 28,
    ) -> Dict[str, Any]:
        """
        Build Qwen image editing workflow.

        Args:
            instruction: Editing instruction (e.g., "make the sky pink")
            image_path: Path to input image (uploaded to ComfyUI)
            mask_path: Optional inpainting mask (uploaded to ComfyUI)
            seed: Random seed (None = random)
            cfg_scale: Classifier-free guidance scale
            steps: Number of diffusion steps

        Returns:
            Complete ComfyUI workflow dict
        """
        logger.info(f"Building Qwen workflow: instruction='{instruction[:50]}...'")

        workflow = self._load_template("qwen")

        # Find nodes by class type and update parameters
        for node_id, node in workflow.items():
            class_type = node.get("class_type", "")

            # Load input image
            if class_type == "LoadImage":
                # Assume first LoadImage is the main image
                if "image" not in node.get("_meta", {}).get("title", "").lower():
                    node["inputs"]["image"] = Path(image_path).name

            # Load mask (if provided)
            if class_type == "LoadImageMask" and mask_path:
                node["inputs"]["image"] = Path(mask_path).name

            # Set instruction prompt
            if class_type == "CLIPTextEncode":
                # Qwen uses instruction as prompt
                node["inputs"]["text"] = instruction

            # Set sampling parameters
            if class_type == "KSampler":
                node["inputs"]["seed"] = seed if seed is not None else -1
                node["inputs"]["cfg"] = cfg_scale
                node["inputs"]["steps"] = steps

            # Set denoise strength for inpainting
            if class_type == "KSampler" and mask_path:
                node["inputs"]["denoise"] = 1.0  # Full denoise for masked areas

        logger.debug(f"Qwen workflow built with {len(workflow)} nodes")
        return workflow

    def validate_workflow(self, workflow: Dict[str, Any]) -> bool:
        """
        Validate workflow structure.

        Args:
            workflow: ComfyUI workflow dict

        Returns:
            True if valid

        Raises:
            ValueError: If workflow is invalid
        """
        if not isinstance(workflow, dict):
            raise ValueError("Workflow must be a dict")

        if not workflow:
            raise ValueError("Workflow is empty")

        # Check that all nodes have required fields
        for node_id, node in workflow.items():
            if not isinstance(node, dict):
                raise ValueError(f"Node {node_id} is not a dict")

            if "class_type" not in node:
                raise ValueError(f"Node {node_id} missing 'class_type'")

            if "inputs" not in node:
                raise ValueError(f"Node {node_id} missing 'inputs'")

        logger.debug(f"Workflow validated: {len(workflow)} nodes")
        return True
