"""
diffbox Python worker entry point.

Communicates with Go backend via stdin/stdout JSON protocol.
"""

import json
import sys
import os
import logging

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(levelname)s [%(name)s] %(message)s',
    stream=sys.stderr
)
logger = logging.getLogger('worker')

# Add parent directory to path for diffsynth import
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from worker.protocol import read_message, send_ready, send_complete, send_error  # noqa: E402


def main():
    """Main worker loop."""
    worker_id = os.environ.get("WORKER_ID", "0")
    models_dir = os.environ.get("DIFFBOX_MODELS_DIR", "./models")
    outputs_dir = os.environ.get("DIFFBOX_OUTPUTS_DIR", "./outputs")

    logger.info(f"Worker {worker_id} starting...")
    logger.info(f"Models dir: {models_dir}")
    logger.info(f"Outputs dir: {outputs_dir}")

    # Lazy import handlers to avoid loading models until needed
    handlers = {}

    def get_handler(job_type: str):
        if job_type not in handlers:
            if job_type == "i2v":
                from worker.i2v import I2VHandler
                handlers[job_type] = I2VHandler(models_dir, outputs_dir)
            elif job_type == "svi":
                from worker.svi import SVIHandler
                handlers[job_type] = SVIHandler(models_dir, outputs_dir)
            elif job_type == "qwen":
                from worker.qwen import QwenHandler
                handlers[job_type] = QwenHandler(models_dir, outputs_dir)
            else:
                raise ValueError(f"Unknown job type: {job_type}")
        return handlers[job_type]

    # Signal ready
    send_ready()

    # Main loop
    while True:
        try:
            msg = read_message()
            if msg is None:
                break

            msg_type = msg.get("type")

            if msg_type == "shutdown":
                logger.info(f"Worker {worker_id} shutting down...")
                break

            elif msg_type == "job":
                job_data = msg.get("data", {})
                job_id = job_data.get("id")
                job_type = job_data.get("type")
                params = job_data.get("params", {})

                logger.info(f"Processing job {job_id} ({job_type})")
                logger.debug(f"Job {job_id} params: {params}")

                try:
                    handler = get_handler(job_type)
                    result = handler.run(job_id, params)
                    send_complete(job_id, result)
                    logger.info(f"Job {job_id} completed successfully")
                except Exception as e:
                    error_msg = f"{type(e).__name__}: {str(e)}"
                    logger.error(f"Job {job_id} failed: {error_msg}")
                    logger.error(f"Job {job_id} parameters: {params}")
                    logger.error("Traceback:", exc_info=True)
                    send_error(job_id, error_msg)

        except json.JSONDecodeError as e:
            logger.error(f"Invalid JSON: {e}")
        except Exception as e:
            logger.error(f"Worker error: {e}", exc_info=True)


if __name__ == "__main__":
    main()
