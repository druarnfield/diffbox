"""
ComfyUI HTTP/WebSocket API client for workflow execution and progress tracking.
"""

import asyncio
import json
import logging
import os
import uuid
from typing import Any, Callable, Dict, Optional

import aiohttp
import websockets
from PIL import Image

logger = logging.getLogger(__name__)


class ComfyUIClient:
    """Async client for ComfyUI HTTP/WebSocket API."""

    def __init__(self, base_url: Optional[str] = None):
        self.base_url = base_url or os.getenv("COMFYUI_URL", "http://localhost:8188")
        self.ws_url = self.base_url.replace("http://", "ws://").replace(
            "https://", "wss://"
        )
        self.client_id = str(uuid.uuid4())
        logger.info(f"ComfyUI client initialized: {self.base_url}")

    async def upload_image(self, image: Image.Image, filename: str) -> str:
        """
        Upload an image to ComfyUI's input directory.

        Args:
            image: PIL Image to upload
            filename: Filename to save as

        Returns:
            Uploaded filename (may be modified by server)
        """
        logger.info(f"Uploading image: {filename}")

        # Convert PIL Image to bytes
        from io import BytesIO

        buffer = BytesIO()
        image.save(buffer, format="PNG")
        buffer.seek(0)

        # Upload via multipart form data
        data = aiohttp.FormData()
        data.add_field(
            "image", buffer, filename=filename, content_type="image/png"
        )

        async with aiohttp.ClientSession() as session:
            async with session.post(
                f"{self.base_url}/upload/image", data=data
            ) as resp:
                if resp.status != 200:
                    error_text = await resp.text()
                    raise RuntimeError(
                        f"Image upload failed ({resp.status}): {error_text}"
                    )

                result = await resp.json()
                uploaded_name = result.get("name", filename)
                logger.info(f"Image uploaded as: {uploaded_name}")
                return uploaded_name

    async def queue_prompt(self, workflow: Dict[str, Any]) -> str:
        """
        Submit a workflow to ComfyUI's queue.

        Args:
            workflow: ComfyUI workflow JSON (node graph)

        Returns:
            Prompt ID for tracking execution
        """
        payload = {"prompt": workflow, "client_id": self.client_id}

        logger.info(f"Queuing workflow with {len(workflow)} nodes")

        async with aiohttp.ClientSession() as session:
            async with session.post(
                f"{self.base_url}/prompt", json=payload
            ) as resp:
                if resp.status != 200:
                    error_text = await resp.text()
                    raise RuntimeError(
                        f"Failed to queue prompt ({resp.status}): {error_text}"
                    )

                result = await resp.json()
                prompt_id = result["prompt_id"]
                logger.info(f"Workflow queued: {prompt_id}")
                return prompt_id

    async def track_progress(
        self,
        prompt_id: str,
        on_progress: Optional[Callable[[float, str], None]] = None,
        timeout: int = 600,
    ) -> Dict[str, Any]:
        """
        Track workflow execution via WebSocket and wait for completion.

        Args:
            prompt_id: Prompt ID from queue_prompt()
            on_progress: Callback for progress updates (progress: 0.0-1.0, stage: str)
            timeout: Maximum seconds to wait for completion

        Returns:
            Execution history with output file paths
        """
        ws_url = f"{self.ws_url}/ws?clientId={self.client_id}"
        logger.info(f"Connecting to WebSocket: {ws_url}")

        try:
            async with websockets.connect(
                ws_url, close_timeout=10
            ) as websocket:
                logger.info("WebSocket connected, waiting for execution...")

                start_time = asyncio.get_event_loop().time()

                while True:
                    # Check timeout
                    if asyncio.get_event_loop().time() - start_time > timeout:
                        raise TimeoutError(
                            f"Workflow execution exceeded {timeout}s timeout"
                        )

                    # Read message with timeout
                    try:
                        message_raw = await asyncio.wait_for(
                            websocket.recv(), timeout=10.0
                        )
                    except asyncio.TimeoutError:
                        # No message in 10s, keep waiting
                        continue

                    # Parse message
                    if not isinstance(message_raw, str):
                        continue

                    try:
                        message = json.loads(message_raw)
                    except json.JSONDecodeError:
                        logger.warning(f"Invalid JSON from WebSocket: {message_raw[:100]}")
                        continue

                    msg_type = message.get("type")

                    if msg_type == "status":
                        # Server status update (queue info, etc.)
                        logger.debug(f"Status: {message.get('data', {})}")

                    elif msg_type == "progress":
                        # Inference progress (current step / max steps)
                        data = message.get("data", {})
                        value = data.get("value", 0)
                        max_val = data.get("max", 1)
                        node = data.get("node")

                        progress = value / max(max_val, 1)
                        stage = f"Step {value}/{max_val}"
                        if node:
                            stage = f"{node}: {stage}"

                        logger.debug(f"Progress: {progress:.1%} - {stage}")

                        if on_progress:
                            on_progress(progress, stage)

                    elif msg_type == "executing":
                        # Node execution status
                        data = message.get("data", {})
                        node = data.get("node")
                        prompt_id_msg = data.get("prompt_id")

                        # Only track our prompt
                        if prompt_id_msg != prompt_id:
                            continue

                        if node is None:
                            # Execution complete (node=null means done)
                            logger.info(f"Workflow {prompt_id} completed")
                            break
                        else:
                            logger.debug(f"Executing node: {node}")

                    elif msg_type == "executed":
                        # Node execution finished (may have outputs)
                        data = message.get("data", {})
                        node = data.get("node")
                        prompt_id_msg = data.get("prompt_id")

                        if prompt_id_msg == prompt_id:
                            logger.debug(f"Node {node} executed successfully")

                    elif msg_type == "execution_error":
                        # Execution failed
                        data = message.get("data", {})
                        error_msg = data.get("exception_message", "Unknown error")
                        node_id = data.get("node_id")
                        raise RuntimeError(
                            f"ComfyUI execution error at node {node_id}: {error_msg}"
                        )

        except websockets.exceptions.WebSocketException as e:
            raise RuntimeError(f"WebSocket connection failed: {e}")

        # Get execution history to retrieve output files
        return await self.get_history(prompt_id)

    async def get_history(self, prompt_id: str) -> Dict[str, Any]:
        """
        Get execution history for a completed prompt.

        Args:
            prompt_id: Prompt ID

        Returns:
            History dict with outputs
        """
        logger.info(f"Fetching history for: {prompt_id}")

        async with aiohttp.ClientSession() as session:
            async with session.get(
                f"{self.base_url}/history/{prompt_id}"
            ) as resp:
                if resp.status != 200:
                    raise RuntimeError(
                        f"Failed to fetch history ({resp.status}): {await resp.text()}"
                    )

                history = await resp.json()

                if prompt_id not in history:
                    raise RuntimeError(f"Prompt {prompt_id} not found in history")

                return history[prompt_id]

    async def download_output(
        self, filename: str, subfolder: str = "", output_type: str = "output"
    ) -> bytes:
        """
        Download output file from ComfyUI.

        Args:
            filename: Output filename
            subfolder: Subfolder within output directory
            output_type: 'output', 'input', or 'temp'

        Returns:
            File contents as bytes
        """
        params = {
            "filename": filename,
            "type": output_type,
        }
        if subfolder:
            params["subfolder"] = subfolder

        logger.info(f"Downloading output: {filename}")

        async with aiohttp.ClientSession() as session:
            async with session.get(
                f"{self.base_url}/view", params=params
            ) as resp:
                if resp.status != 200:
                    raise RuntimeError(
                        f"Failed to download output ({resp.status}): {await resp.text()}"
                    )

                return await resp.read()

    def extract_outputs(self, history: Dict[str, Any]) -> Dict[str, Any]:
        """
        Extract output file paths from execution history.

        Args:
            history: History dict from get_history()

        Returns:
            Dict mapping output type to file info
        """
        outputs = {}

        history_outputs = history.get("outputs", {})

        for node_id, node_output in history_outputs.items():
            # Videos (from VHS nodes)
            if "gifs" in node_output:
                for video_info in node_output["gifs"]:
                    outputs["video"] = {
                        "filename": video_info["filename"],
                        "subfolder": video_info.get("subfolder", ""),
                        "type": video_info.get("type", "output"),
                    }
                    logger.info(f"Found video output: {video_info['filename']}")

            # Images
            if "images" in node_output:
                for image_info in node_output["images"]:
                    outputs["image"] = {
                        "filename": image_info["filename"],
                        "subfolder": image_info.get("subfolder", ""),
                        "type": image_info.get("type", "output"),
                    }
                    logger.info(f"Found image output: {image_info['filename']}")

        if not outputs:
            logger.warning("No outputs found in history")

        return outputs

    async def execute_workflow(
        self,
        workflow: Dict[str, Any],
        on_progress: Optional[Callable[[float, str], None]] = None,
        timeout: int = 600,
    ) -> Dict[str, Any]:
        """
        Execute a complete workflow: queue → track → get outputs.

        Args:
            workflow: ComfyUI workflow JSON
            on_progress: Progress callback
            timeout: Execution timeout in seconds

        Returns:
            Dict with 'history' and 'outputs' keys
        """
        # Queue the workflow
        prompt_id = await self.queue_prompt(workflow)

        # Track execution
        history = await self.track_progress(prompt_id, on_progress, timeout)

        # Extract output file info
        outputs = self.extract_outputs(history)

        return {"history": history, "outputs": outputs, "prompt_id": prompt_id}

    async def health_check(self) -> bool:
        """
        Check if ComfyUI server is responding.

        Returns:
            True if healthy, False otherwise
        """
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(
                    self.base_url, timeout=aiohttp.ClientTimeout(total=5)
                ) as resp:
                    return resp.status == 200
        except Exception as e:
            logger.warning(f"ComfyUI health check failed: {e}")
            return False
