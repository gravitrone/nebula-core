"""Shared helpers for bulk import parsing and normalization."""

import csv
import io
import json
from typing import Any


def coerce_list(value: Any) -> list[str]:
    """Normalize an input value into a list of strings.

    Args:
        value: Input value which may be None, list, or string.

    Returns:
        A list of non-empty, stripped strings.
    """

    if value is None:
        return []
    if isinstance(value, list):
        return [str(v).strip() for v in value if str(v).strip()]
    if isinstance(value, str):
        return [v.strip() for v in value.split(",") if v.strip()]
    return [str(value).strip()]


def coerce_json(value: Any) -> dict[str, Any]:
    """Normalize an input value into a JSON-compatible dict.

    Args:
        value: Input value that can be a dict, JSON string, or None.

    Returns:
        A dict representation of the input value.
    """

    if value is None or value == "":
        return {}
    if isinstance(value, dict):
        return value
    if isinstance(value, str):
        try:
            parsed = json.loads(value)
        except json.JSONDecodeError as exc:
            raise ValueError("Invalid JSON object") from exc
        if isinstance(parsed, dict):
            return parsed
        return {}
    return {}


def coerce_text(value: Any) -> str | None:
    """Normalize an input value into a trimmed string.

    Args:
        value: Input value that may be None or any type.

    Returns:
        A stripped string or None if empty.
    """

    if value is None:
        return None
    text = str(value).strip()
    return text or None


def parse_csv_rows(data: str) -> list[dict[str, Any]]:
    """Parse CSV content into cleaned row dictionaries.

    Args:
        data: CSV string content.

    Returns:
        A list of dictionaries, one per non-empty row.
    """

    reader = csv.DictReader(io.StringIO(data))
    rows: list[dict[str, Any]] = []
    for row in reader:
        cleaned = {k: v for k, v in row.items() if k is not None}
        if any(v is not None and str(v).strip() != "" for v in cleaned.values()):
            rows.append(cleaned)
    return rows


def extract_items(
    fmt: str, data: str | None, items: list[dict[str, Any]] | None
) -> list[dict[str, Any]]:
    """Extract items from JSON payload or CSV data.

    Args:
        fmt: Input format, either "json" or "csv".
        data: CSV content when format is csv.
        items: JSON items when format is json.

    Returns:
        A list of item dictionaries ready for normalization.

    Raises:
        ValueError: If format is invalid or required data is missing.
    """

    fmt = (fmt or "json").lower()
    if fmt not in {"json", "csv"}:
        raise ValueError("Format must be json or csv")
    if fmt == "csv":
        if not data:
            raise ValueError("CSV data is required")
        rows = parse_csv_rows(data)
        if not rows:
            raise ValueError("CSV data is empty")
        return rows
    if not items:
        raise ValueError("Items are required for JSON import")
    return items


def merge_defaults(item: dict[str, Any], defaults: dict[str, Any] | None) -> dict[str, Any]:
    """Merge a default dictionary into an item dictionary.

    Args:
        item: Item dictionary to normalize.
        defaults: Default values to apply when missing.

    Returns:
        A new merged dictionary.
    """

    if not defaults:
        return dict(item)
    merged = dict(defaults)
    merged.update(item)
    return merged


def normalize_entity(item: dict[str, Any], defaults: dict[str, Any] | None) -> dict:
    """Normalize an entity payload for bulk import.

    Args:
        item: Raw entity dictionary.
        defaults: Default values applied to missing fields.

    Returns:
        Normalized entity dictionary.

    Raises:
        ValueError: If required fields are missing.
    """

    merged = merge_defaults(item, defaults)
    if "metadata" in merged:
        raise ValueError("Entity metadata is not supported")
    name = coerce_text(merged.get("name"))
    entity_type = coerce_text(merged.get("type"))
    if not name or not entity_type:
        raise ValueError("Entity name and type are required")
    scopes = coerce_list(merged.get("scopes")) or coerce_list((defaults or {}).get("scopes"))
    if not scopes:
        scopes = ["public"]
    return {
        "name": name,
        "type": entity_type,
        "status": coerce_text(merged.get("status")) or "active",
        "scopes": scopes,
        "tags": coerce_list(merged.get("tags")),
        "source_path": coerce_text(merged.get("source_path")),
    }


def normalize_context(item: dict[str, Any], defaults: dict[str, Any] | None) -> dict:
    """Normalize a context payload for bulk import.

    Args:
        item: Raw context dictionary.
        defaults: Default values applied to missing fields.

    Returns:
        Normalized context dictionary.

    Raises:
        ValueError: If required fields are missing.
    """

    merged = merge_defaults(item, defaults)
    if "metadata" in merged:
        raise ValueError("Context metadata is not supported")
    title = coerce_text(merged.get("title"))
    source_type = coerce_text(merged.get("source_type"))
    if not title or not source_type:
        raise ValueError("Context title and source_type are required")
    scopes = coerce_list(merged.get("scopes")) or coerce_list((defaults or {}).get("scopes"))
    if not scopes:
        scopes = ["public"]
    url = coerce_text(merged.get("url"))
    if url:
        url = url.strip()
        if not (url.startswith("http://") or url.startswith("https://")):
            raise ValueError("Context URL must start with http:// or https://")
    return {
        "title": title,
        "url": url,
        "source_type": source_type,
        "content": coerce_text(merged.get("content")),
        "scopes": scopes,
        "tags": coerce_list(merged.get("tags")),
    }


def normalize_relationship(item: dict[str, Any], defaults: dict[str, Any] | None) -> dict:
    """Normalize a relationship payload for bulk import.

    Args:
        item: Raw relationship dictionary.
        defaults: Default values applied to missing fields.

    Returns:
        Normalized relationship dictionary.

    Raises:
        ValueError: If required fields are missing.
    """

    merged = merge_defaults(item, defaults)
    source_type = coerce_text(merged.get("source_type"))
    source_id = coerce_text(merged.get("source_id"))
    target_type = coerce_text(merged.get("target_type"))
    target_id = coerce_text(merged.get("target_id"))
    relationship_type = coerce_text(merged.get("relationship_type"))
    if not all([source_type, source_id, target_type, target_id, relationship_type]):
        raise ValueError("Relationship source, target, and type are required")
    return {
        "source_type": source_type,
        "source_id": source_id,
        "target_type": target_type,
        "target_id": target_id,
        "relationship_type": relationship_type,
        "properties": coerce_json(merged.get("properties")),
    }


def normalize_job(item: dict[str, Any], defaults: dict[str, Any] | None) -> dict:
    """Normalize a job payload for bulk import.

    Args:
        item: Raw job dictionary.
        defaults: Default values applied to missing fields.

    Returns:
        Normalized job dictionary.

    Raises:
        ValueError: If required fields are missing.
    """

    merged = merge_defaults(item, defaults)
    title = coerce_text(merged.get("title"))
    if not title:
        raise ValueError("Job title is required")
    return {
        "title": title,
        "description": coerce_text(merged.get("description")),
        "job_type": coerce_text(merged.get("job_type")),
        "assigned_to": coerce_text(merged.get("assigned_to")),
        "agent_id": coerce_text(merged.get("agent_id")),
        "priority": coerce_text(merged.get("priority")) or "medium",
        "parent_job_id": coerce_text(merged.get("parent_job_id")),
        "due_at": coerce_text(merged.get("due_at")),
    }
