"""
diffbox Python worker entry point.

Communicates with Go backend via stdin/stdout JSON protocol.
"""

import json
import sys
import os
import traceback
from typing import Any

# Add parent directory to path for diffsynth import
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from worker.protocol import send_message, read_message, send_ready, send_progress, send_complete, send_error


def main():
    """Main worker loop."""
    worker_id = os.environ.get("WORKER_ID", "0")
    models_dir = os.environ.get("DIFFBOX_MODELS_DIR", "./models")
    outputs_dir = os.environ.get("DIFFBOX_OUTPUTS_DIR", "./outputs")

    # Log to stderr (stdout is for JSON protocol)
    print(f"Worker {worker_id} starting...", file=sys.stderr)
    print(f"Models dir: {models_dir}", file=sys.stderr)
    print(f"Outputs dir: {outputs_dir}", file=sys.stderr)

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
                print(f"Worker {worker_id} shutting down...", file=sys.stderr)
                break

            elif msg_type == "job":
                job_data = msg.get("data", {})
                job_id = job_data.get("id")
                job_type = job_data.get("type")
                params = job_data.get("params", {})

                print(f"Worker {worker_id} processing job {job_id} ({job_type})", file=sys.stderr)

                try:
                    handler = get_handler(job_type)
                    result = handler.run(job_id, params)
                    send_complete(job_id, result)
                except Exception as e:
                    print(f"Job {job_id} failed: {e}", file=sys.stderr)
                    traceback.print_exc(file=sys.stderr)
                    send_error(job_id, str(e))

        except json.JSONDecodeError as e:
            print(f"Invalid JSON: {e}", file=sys.stderr)
        except Exception as e:
            print(f"Worker error: {e}", file=sys.stderr)
            traceback.print_exc(file=sys.stderr)


if __name__ == "__main__":
    main()
