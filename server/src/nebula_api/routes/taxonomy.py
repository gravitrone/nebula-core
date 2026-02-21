"""Taxonomy management API routes."""

# Standard Library
import json
from pathlib import Path
from typing import Any
from uuid import UUID

# Third-Party
from asyncpg import UniqueViolationError
from fastapi import APIRouter, Depends, Request
from pydantic import BaseModel

# Local
from nebula_api.auth import require_auth
from nebula_api.response import api_error, success
from nebula_mcp.enums import EnumRegistry, load_enums
from nebula_mcp.query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "queries")

router = APIRouter()
ADMIN_SCOPE_NAMES = {"admin"}

KIND_MAP = {
    "scopes": {
        "list": "taxonomy/list_scopes",
        "create": "taxonomy/create_scope",
        "update": "taxonomy/update_scope",
        "set_active": "taxonomy/set_scope_active",
        "usage": "taxonomy/count_scope_usage",
        "supports": set(),
    },
    "entity-types": {
        "list": "taxonomy/list_entity_types",
        "create": "taxonomy/create_entity_type",
        "update": "taxonomy/update_entity_type",
        "set_active": "taxonomy/set_entity_type_active",
        "usage": "taxonomy/count_entity_type_usage",
        "supports": set(),
    },
    "relationship-types": {
        "list": "taxonomy/list_relationship_types",
        "create": "taxonomy/create_relationship_type",
        "update": "taxonomy/update_relationship_type",
        "set_active": "taxonomy/set_relationship_type_active",
        "usage": "taxonomy/count_relationship_type_usage",
        "supports": {"is_symmetric"},
    },
    "log-types": {
        "list": "taxonomy/list_log_types",
        "create": "taxonomy/create_log_type",
        "update": "taxonomy/update_log_type",
        "set_active": "taxonomy/set_log_type_active",
        "usage": "taxonomy/count_log_type_usage",
        "supports": {"value_schema"},
    },
}

ROW_QUERY_BY_KIND = {
    "scopes": "taxonomy/get_scope_row",
    "entity-types": "taxonomy/get_entity_type_row",
    "relationship-types": "taxonomy/get_relationship_type_row",
    "log-types": "taxonomy/get_log_type_row",
}


def _require_uuid(value: str, label: str) -> None:
    """Handle require uuid.

    Args:
        value: Input parameter for _require_uuid.
        label: Input parameter for _require_uuid.
    """

    try:
        UUID(str(value))
    except ValueError:
        api_error("INVALID_INPUT", f"Invalid {label} id", 400)


def _kind_or_error(kind: str) -> dict[str, Any]:
    """Handle kind or error.

    Args:
        kind: Input parameter for _kind_or_error.

    Returns:
        Result value from the operation.
    """

    cfg = KIND_MAP.get(kind)
    if cfg is None:
        api_error("INVALID_INPUT", f"Unknown taxonomy kind: {kind}", 400)
    return cfg


def _require_admin_scope(auth: dict, enums: EnumRegistry) -> None:
    """Handle require admin scope.

    Args:
        auth: Input parameter for _require_admin_scope.
        enums: Input parameter for _require_admin_scope.
    """

    scope_ids = set(auth.get("scopes", []))
    allowed_ids = {
        enums.scopes.name_to_id.get(name)
        for name in ADMIN_SCOPE_NAMES
        if enums.scopes.name_to_id.get(name)
    }
    if not scope_ids.intersection(allowed_ids):
        api_error("FORBIDDEN", "Admin scope required", 403)


async def _refresh_enums(request: Request) -> None:
    """Handle refresh enums.

    Args:
        request: Input parameter for _refresh_enums.
    """

    request.app.state.enums = await load_enums(request.app.state.pool)


async def _usage_count(pool: Any, cfg: dict[str, Any], item_id: str) -> int:
    """Handle usage count.

    Args:
        pool: Input parameter for _usage_count.
        cfg: Input parameter for _usage_count.
        item_id: Input parameter for _usage_count.

    Returns:
        Result value from the operation.
    """

    value = await pool.fetchval(QUERIES[cfg["usage"]], item_id)
    if value is None:
        return 0
    return int(value)


async def _fetch_taxonomy_row(
    pool: Any, kind: str, item_id: str
) -> dict[str, Any] | None:
    """Handle fetch taxonomy row.

    Args:
        pool: Input parameter for _fetch_taxonomy_row.
        kind: Input parameter for _fetch_taxonomy_row.
        item_id: Input parameter for _fetch_taxonomy_row.

    Returns:
        Result value from the operation.
    """

    query = ROW_QUERY_BY_KIND[kind]
    row = await pool.fetchrow(
        QUERIES[query],
        item_id,
    )
    return dict(row) if row else None


async def _ensure_can_archive(
    pool: Any, kind: str, cfg: dict[str, Any], item_id: str
) -> None:
    """Handle ensure can archive.

    Args:
        pool: Input parameter for _ensure_can_archive.
        kind: Input parameter for _ensure_can_archive.
        cfg: Input parameter for _ensure_can_archive.
        item_id: Input parameter for _ensure_can_archive.
    """

    usage = await _usage_count(pool, cfg, item_id)
    if usage > 0:
        api_error(
            "CONFLICT",
            f"Cannot archive {kind} entry while it is referenced ({usage} records)",
            409,
        )


class TaxonomyCreateBody(BaseModel):
    """Payload for creating taxonomy items."""

    name: str
    description: str | None = None
    metadata: dict[str, Any] | None = None
    is_symmetric: bool | None = None
    value_schema: dict[str, Any] | None = None


class TaxonomyUpdateBody(BaseModel):
    """Payload for updating taxonomy items."""

    name: str | None = None
    description: str | None = None
    metadata: dict[str, Any] | None = None
    is_symmetric: bool | None = None
    value_schema: dict[str, Any] | None = None


def _validate_payload(
    kind: str,
    payload: TaxonomyCreateBody | TaxonomyUpdateBody,
) -> None:
    """Handle validate payload.

    Args:
        kind: Input parameter for _validate_payload.
        payload: Input parameter for _validate_payload.
    """

    supports = _kind_or_error(kind)["supports"]
    if payload.is_symmetric is not None and "is_symmetric" not in supports:
        api_error(
            "INVALID_INPUT",
            "is_symmetric is only valid for relationship-types",
            400,
        )
    if payload.value_schema is not None and "value_schema" not in supports:
        api_error("INVALID_INPUT", "value_schema is only valid for log-types", 400)


@router.get("/{kind}")
async def list_taxonomy(
    kind: str,
    request: Request,
    include_inactive: bool = False,
    search: str | None = None,
    limit: int = 200,
    offset: int = 0,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """List taxonomy rows by kind."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    cfg = _kind_or_error(kind)
    limit = max(1, min(limit, 1000))
    offset = max(0, offset)
    rows = await pool.fetch(
        QUERIES[cfg["list"]],
        include_inactive,
        search,
        limit,
        offset,
    )
    return success([dict(r) for r in rows])


@router.post("/{kind}")
async def create_taxonomy(
    kind: str,
    payload: TaxonomyCreateBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Create taxonomy row by kind."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    cfg = _kind_or_error(kind)
    _validate_payload(kind, payload)

    name = payload.name.strip()
    if not name:
        api_error("INVALID_INPUT", "Name required", 400)

    try:
        if kind in {"scopes", "entity-types"}:
            row = await pool.fetchrow(
                QUERIES[cfg["create"]],
                name,
                payload.description,
                json.dumps(payload.metadata or {}),
            )
        elif kind == "relationship-types":
            row = await pool.fetchrow(
                QUERIES[cfg["create"]],
                name,
                payload.description,
                payload.is_symmetric,
                json.dumps(payload.metadata or {}),
            )
        else:
            row = await pool.fetchrow(
                QUERIES[cfg["create"]],
                name,
                payload.description,
                json.dumps(payload.value_schema) if payload.value_schema else None,
            )
    except UniqueViolationError:
        api_error("DUPLICATE", f"{kind} entry already exists", 409)

    await _refresh_enums(request)
    return success(dict(row))


@router.patch("/{kind}/{item_id}")
async def update_taxonomy(
    kind: str,
    item_id: str,
    payload: TaxonomyUpdateBody,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Update taxonomy row by kind."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    cfg = _kind_or_error(kind)
    _validate_payload(kind, payload)
    _require_uuid(item_id, kind)
    current = await _fetch_taxonomy_row(pool, kind, item_id)
    if current is None:
        api_error("NOT_FOUND", f"{kind} entry not found", 404)

    name = payload.name.strip() if payload.name is not None else None
    if payload.name is not None and not name:
        api_error("INVALID_INPUT", "Name cannot be empty", 400)
    if current["is_builtin"] and name is not None and name != current["name"]:
        api_error("CONFLICT", "Built-in taxonomy names are immutable", 409)

    try:
        if kind in {"scopes", "entity-types"}:
            row = await pool.fetchrow(
                QUERIES[cfg["update"]],
                item_id,
                name,
                payload.description,
                json.dumps(payload.metadata) if payload.metadata is not None else None,
            )
        elif kind == "relationship-types":
            row = await pool.fetchrow(
                QUERIES[cfg["update"]],
                item_id,
                name,
                payload.description,
                payload.is_symmetric,
                json.dumps(payload.metadata) if payload.metadata is not None else None,
            )
        else:
            row = await pool.fetchrow(
                QUERIES[cfg["update"]],
                item_id,
                name,
                payload.description,
                (
                    json.dumps(payload.value_schema)
                    if payload.value_schema is not None
                    else None
                ),
            )
    except UniqueViolationError:
        api_error("DUPLICATE", f"{kind} entry already exists", 409)

    if row is None:
        api_error("NOT_FOUND", f"{kind} entry not found", 404)

    await _refresh_enums(request)
    return success(dict(row))


@router.post("/{kind}/{item_id}/archive")
async def archive_taxonomy(
    kind: str,
    item_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Archive taxonomy row by kind."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    cfg = _kind_or_error(kind)
    _require_uuid(item_id, kind)
    await _ensure_can_archive(pool, kind, cfg, item_id)

    row = await pool.fetchrow(QUERIES[cfg["set_active"]], item_id, False)
    if row is None:
        api_error(
            "NOT_FOUND",
            f"{kind} entry not found or cannot archive built-in entry",
            404,
        )

    await _refresh_enums(request)
    return success(dict(row))


@router.post("/{kind}/{item_id}/activate")
async def activate_taxonomy(
    kind: str,
    item_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Activate taxonomy row by kind."""

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)
    cfg = _kind_or_error(kind)
    _require_uuid(item_id, kind)

    row = await pool.fetchrow(QUERIES[cfg["set_active"]], item_id, True)
    if row is None:
        api_error("NOT_FOUND", f"{kind} entry not found", 404)

    await _refresh_enums(request)
    return success(dict(row))
