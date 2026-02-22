"""Unit coverage for log/file response payload normalizers."""

# Third-Party
import pytest

# Local
from nebula_api.routes.files import _normalize_file_payload
from nebula_api.routes.logs import _normalize_log_payload


pytestmark = pytest.mark.unit


def test_normalize_log_payload_parses_json_string_fields():
    """Log normalizer should parse stringified JSON object fields."""

    payload = {
        "id": "log-1",
        "value": '{"text":"hello"}',
        "metadata": '{"owner":"alxx"}',
    }

    normalized = _normalize_log_payload(payload)

    assert normalized["value"] == {"text": "hello"}
    assert normalized["metadata"] == {"owner": "alxx"}


def test_normalize_log_payload_falls_back_on_invalid_or_non_object():
    """Log normalizer should coerce invalid and non-object fields to empty objects."""

    payload = {
        "id": "log-2",
        "value": "[1,2,3]",
        "metadata": "not-json",
    }

    normalized = _normalize_log_payload(payload)

    assert normalized["value"] == {}
    assert normalized["metadata"] == {}


def test_normalize_file_payload_parses_stringified_metadata():
    """File normalizer should parse object metadata encoded as JSON text."""

    payload = {"id": "file-1", "metadata": '{"owner":"alxx","source":"upload"}'}

    normalized = _normalize_file_payload(payload)

    assert normalized["metadata"] == {"owner": "alxx", "source": "upload"}


def test_normalize_file_payload_falls_back_for_non_object_metadata():
    """File normalizer should coerce non-object metadata payloads to empty objects."""

    list_payload = {"id": "file-2", "metadata": ["bad", "shape"]}
    scalar_payload = {"id": "file-3", "metadata": "7"}

    assert _normalize_file_payload(list_payload)["metadata"] == {}
    assert _normalize_file_payload(scalar_payload)["metadata"] == {}
