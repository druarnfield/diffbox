"""
JSON protocol for communicating with Go backend via stdin/stdout.
"""

import json
import sys
from typing import Any, Optional


def send_message(msg_type: str, job_id: Optional[str] = None, data: Any = None):
    """Send a JSON message to stdout."""
    msg = {"type": msg_type}
    if job_id:
        msg["job_id"] = job_id
    if data is not None:
        msg["data"] = data

    # Write to stdout as single line JSON
    print(json.dumps(msg), flush=True)


def read_message() -> Optional[dict]:
    """Read a JSON message from stdin."""
    line = sys.stdin.readline()
    if not line:
        return None
    return json.loads(line.strip())


def send_ready():
    """Signal that worker is ready to accept jobs."""
    send_message("ready")


def send_progress(
    job_id: str, progress: float, stage: str, preview: Optional[str] = None
):
    """Send job progress update."""
    data = {
        "job_id": job_id,
        "progress": progress,
        "stage": stage,
    }
    if preview:
        data["preview"] = preview
    send_message("progress", job_id=job_id, data=data)


def send_complete(job_id: str, output: dict):
    """Send job completion."""
    data = {
        "job_id": job_id,
        "status": "completed",
        "output": output,
    }
    send_message("complete", job_id=job_id, data=data)


def send_error(job_id: str, error: str):
    """Send job error."""
    data = {
        "job_id": job_id,
        "status": "failed",
        "error": error,
    }
    send_message("error", job_id=job_id, data=data)
