"""
Chat handler using vLLM for Dolphin-Mistral model.
"""

import logging
from pathlib import Path
from typing import Optional

from worker.protocol import send_progress

logger = logging.getLogger(__name__)


class ChatHandler:
    """Handler for chat inference using vLLM."""

    def __init__(self, models_dir: str, outputs_dir: str):
        self.models_dir = Path(models_dir)
        self.outputs_dir = Path(outputs_dir)
        self.llm = None

    def _load_model(self):
        """Lazy load the vLLM model."""
        if self.llm is not None:
            return

        from vllm import LLM, SamplingParams

        logger.info("Loading Dolphin-Mistral model with vLLM...")

        # vLLM will load from the models directory
        model_path = str(self.models_dir / "dolphin-mistral-24b")

        self.llm = LLM(
            model=model_path,
            tensor_parallel_size=1,
            dtype="auto",
            trust_remote_code=True,
        )

        logger.info("Dolphin-Mistral model loaded successfully")

    def run(self, job_id: str, params: dict) -> dict:
        """
        Execute chat inference.

        Args:
            job_id: Unique job identifier
            params: Dict with keys:
                - messages: List of chat messages [{"role": "user/assistant/system", "content": "..."}]
                - max_tokens: Max tokens to generate (default: 512)
                - temperature: Sampling temperature (default: 0.7)
                - top_p: Nucleus sampling (default: 0.9)
                - stream: Whether to stream response (default: False)

        Returns:
            Dict with 'response' key containing generated text
        """
        from vllm import SamplingParams

        send_progress(job_id, 0.1, "Loading model...")
        self._load_model()

        # Extract parameters
        messages = params.get("messages", [])
        max_tokens = params.get("max_tokens", 512)
        temperature = params.get("temperature", 0.7)
        top_p = params.get("top_p", 0.9)

        if not messages:
            raise ValueError("No messages provided")

        send_progress(job_id, 0.3, "Preparing prompt...")

        # Format messages using ChatML format (standard for Mistral-based models)
        prompt = self._format_messages(messages)

        logger.info(f"Generated prompt length: {len(prompt)} chars")

        send_progress(job_id, 0.5, "Generating response...")

        # Configure sampling
        sampling_params = SamplingParams(
            temperature=temperature,
            top_p=top_p,
            max_tokens=max_tokens,
        )

        # Generate
        outputs = self.llm.generate([prompt], sampling_params)

        send_progress(job_id, 0.9, "Processing output...")

        # Extract response
        if not outputs or not outputs[0].outputs:
            raise RuntimeError("No output generated from model")

        response_text = outputs[0].outputs[0].text

        logger.info(f"Generated {len(response_text)} chars")

        return {
            "response": response_text,
            "tokens": len(response_text.split()),
            "finish_reason": outputs[0].outputs[0].finish_reason,
        }

    def _format_messages(self, messages: list) -> str:
        """
        Format messages using ChatML format for Mistral models.

        ChatML format:
        <|im_start|>system
        System message<|im_end|>
        <|im_start|>user
        User message<|im_end|>
        <|im_start|>assistant
        Assistant response<|im_end|>
        """
        formatted = []

        for msg in messages:
            role = msg.get("role", "user")
            content = msg.get("content", "")
            formatted.append(f"<|im_start|>{role}\n{content}<|im_end|>")

        # Add assistant turn start
        formatted.append("<|im_start|>assistant\n")

        return "\n".join(formatted)
