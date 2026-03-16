"""Unit tests for bulk import normalization helpers."""

# Third-Party
import pytest

# Local
from nebula_mcp.imports import (
    coerce_json,
    coerce_list,
    coerce_text,
    extract_items,
    merge_defaults,
    normalize_context,
    normalize_entity,
    normalize_job,
    normalize_relationship,
    parse_csv_rows,
)


def test_coerce_list_handles_none():
    """None should normalize to empty list."""

    assert coerce_list(None) == []


def test_coerce_list_handles_list_and_strips_values():
    """List values should be stringified and stripped."""

    assert coerce_list([" a ", "", 42, "   "]) == ["a", "42"]


def test_coerce_list_handles_csv_string():
    """Comma-separated strings should split into trimmed entries."""

    assert coerce_list("alpha, beta, ,gamma") == ["alpha", "beta", "gamma"]


def test_coerce_list_handles_scalar_value():
    """Non-list/non-string values should become single-item lists."""

    assert coerce_list(7) == ["7"]


def test_coerce_json_handles_empty_values():
    """None and empty strings should normalize to empty object."""

    assert coerce_json(None) == {}
    assert coerce_json("") == {}


def test_coerce_json_handles_dict_and_json_string():
    """Dicts and JSON strings should be decoded as-is."""

    assert coerce_json({"a": 1}) == {"a": 1}
    assert coerce_json('{"b": 2}') == {"b": 2}


def test_coerce_json_non_object_json_string_returns_empty_object():
    """JSON strings that decode to non-objects should normalize to empty objects."""

    assert coerce_json('["a", "b"]') == {}
    assert coerce_json('"text"') == {}


def test_coerce_json_rejects_malformed_json_string():
    """Malformed JSON input should fail with a deterministic validation error."""

    with pytest.raises(ValueError, match="Invalid JSON object"):
        coerce_json("{bad")


def test_coerce_json_non_string_non_dict_returns_empty_object():
    """Unsupported input types should normalize to empty object."""

    assert coerce_json(123) == {}


def test_coerce_text_strips_or_returns_none():
    """Text coercion should trim strings and map empty values to None."""

    assert coerce_text("  hello  ") == "hello"
    assert coerce_text("  ") is None
    assert coerce_text(None) is None


def test_parse_csv_rows_skips_empty_rows():
    """CSV parser should drop fully empty lines."""

    rows = parse_csv_rows("name,type\nalpha,project\n,\n")
    assert rows == [{"name": "alpha", "type": "project"}]


def test_extract_items_rejects_invalid_format():
    """Unsupported import formats should raise clear errors."""

    with pytest.raises(ValueError, match="Format must be json or csv"):
        extract_items("yaml", None, None)


def test_extract_items_requires_csv_data():
    """CSV mode should require data payload."""

    with pytest.raises(ValueError, match="CSV data is required"):
        extract_items("csv", None, None)


def test_extract_items_rejects_empty_csv():
    """CSV mode should reject content that yields no rows."""

    with pytest.raises(ValueError, match="CSV data is empty"):
        extract_items("csv", "name,type\n,\n", None)


def test_extract_items_returns_csv_rows():
    """CSV mode should return parsed non-empty rows."""

    rows = extract_items("csv", "name,type\nalpha,project\n", None)
    assert rows == [{"name": "alpha", "type": "project"}]


def test_extract_items_requires_json_items():
    """JSON mode should require non-empty items."""

    with pytest.raises(ValueError, match="Items are required for JSON import"):
        extract_items("json", None, [])


def test_extract_items_returns_json_items():
    """JSON mode should return provided items unchanged."""

    items = [{"name": "alpha"}, {"name": "beta"}]
    assert extract_items("json", None, items) == items


def test_merge_defaults_without_defaults_returns_copy():
    """Missing defaults should still return a copied mapping."""

    original = {"name": "alpha"}
    merged = merge_defaults(original, None)
    assert merged == original
    assert merged is not original


def test_merge_defaults_overrides_with_item_values():
    """Item keys should override defaults in merged output."""

    merged = merge_defaults({"name": "alpha"}, {"status": "active", "name": "default"})
    assert merged == {"status": "active", "name": "alpha"}


def test_normalize_entity_requires_name_and_type():
    """Entity import rows must include required fields."""

    with pytest.raises(ValueError, match="Entity name and type are required"):
        normalize_entity({"name": "alpha"}, None)


def test_normalize_entity_applies_defaults_and_coercions():
    """Entity normalization should set defaults and drop extra fields."""

    result = normalize_entity(
        {"name": "alpha", "type": "project"},
        None,
    )
    assert result["status"] == "active"
    assert result["scopes"] == ["public"]


def test_normalize_entity_rejects_metadata_payloads():
    """Entity metadata should be rejected in bulk imports."""

    with pytest.raises(ValueError, match="Entity metadata is not supported"):
        normalize_entity(
            {"name": "alpha", "type": "project", "metadata": '{"ok":true}'},
            None,
        )


def test_normalize_context_requires_title_and_source_type():
    """Context rows must include title and source_type."""

    with pytest.raises(ValueError, match="Context title and source_type are required"):
        normalize_context({"title": "x"}, None)


def test_normalize_context_rejects_invalid_url():
    """Context URLs must be HTTP/HTTPS."""

    with pytest.raises(ValueError, match="Context URL must start with http:// or https://"):
        normalize_context(
            {"title": "x", "source_type": "note", "url": "file:///tmp/x"},
            None,
        )


def test_normalize_context_defaults_scopes_to_public():
    """Context normalization should default scopes to public."""

    result = normalize_context({"title": "x", "source_type": "note"}, None)
    assert result["scopes"] == ["public"]


def test_normalize_context_rejects_metadata_payloads():
    """Context metadata should be rejected in bulk imports."""

    with pytest.raises(ValueError, match="Context metadata is not supported"):
        normalize_context(
            {"title": "x", "source_type": "note", "metadata": {"x": 1}},
            None,
        )


def test_normalize_relationship_requires_core_fields():
    """Relationship rows must include source, target, and type."""

    with pytest.raises(ValueError, match="Relationship source, target, and type are required"):
        normalize_relationship({"source_type": "entity"}, None)


def test_normalize_relationship_success_parses_properties():
    """Relationship normalization should parse properties json."""

    result = normalize_relationship(
        {
            "source_type": "entity",
            "source_id": "1",
            "target_type": "entity",
            "target_id": "2",
            "relationship_type": "related-to",
            "properties": '{"k":"v"}',
        },
        None,
    )
    assert result["properties"] == {"k": "v"}


def test_normalize_job_requires_title():
    """Job rows must include title."""

    with pytest.raises(ValueError, match="Job title is required"):
        normalize_job({}, None)


def test_normalize_job_defaults_priority():
    """Job normalization should default missing priority to medium."""

    result = normalize_job({"title": "ship it"}, None)
    assert result["priority"] == "medium"
