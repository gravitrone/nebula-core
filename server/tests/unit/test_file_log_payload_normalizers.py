"""Unit coverage for log/file response payload normalizers."""

# Third-Party
import pytest

# Local
from nebula_api.routes.files import _normalize_file_payload
from nebula_api.routes.logs import _normalize_log_payload

pytestmark = pytest.mark.unit


def test_normalize_log_payload_defaults_none_fields_to_empty_string():
    """Log normalizer should default None content and notes to empty strings."""

    payload = {
        "id": "log-1",
        "content": None,
        "notes": None,
    }

    normalized = _normalize_log_payload(payload)

    assert normalized["content"] == ""
    assert normalized["notes"] == ""


def test_normalize_log_payload_preserves_existing_text_fields():
    """Log normalizer should preserve existing text values."""

    payload = {
        "id": "log-2",
        "content": "some content",
        "notes": "some notes",
    }

    normalized = _normalize_log_payload(payload)

    assert normalized["content"] == "some content"
    assert normalized["notes"] == "some notes"


def test_normalize_file_payload_defaults_none_notes_to_empty_string():
    """File normalizer should default None notes to empty string."""

    payload = {"id": "file-1", "notes": None}

    normalized = _normalize_file_payload(payload)

    assert normalized["notes"] == ""


def test_normalize_file_payload_preserves_existing_notes():
    """File normalizer should preserve existing notes text."""

    payload = {"id": "file-2", "notes": "owner: alxx"}

    normalized = _normalize_file_payload(payload)

    assert normalized["notes"] == "owner: alxx"
