"""Tests for worker protocol."""

import json
from io import StringIO
from unittest.mock import patch

from worker.protocol import read_message


def test_read_message_valid():
    """Test reading a valid JSON message."""
    msg = {"type": "job", "job_id": "123", "params": {"prompt": "test"}}
    input_data = json.dumps(msg) + "\n"

    with patch("sys.stdin", StringIO(input_data)):
        result = read_message()

    assert result == msg


def test_read_message_empty():
    """Test reading from empty input."""
    with patch("sys.stdin", StringIO("")):
        result = read_message()

    assert result is None
