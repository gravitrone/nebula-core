"""API key management and login routes."""

# Standard Library
from pathlib import Path
from typing import Any
from uuid import UUID

# Third-Party
from fastapi import APIRouter, Depends, Request
from pydantic import BaseModel

# Local
from nebula_api.auth import generate_api_key, require_auth
from nebula_api.response import api_error, success
from nebula_mcp.enums import require_entity_type, require_scopes, require_status
from nebula_mcp.query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[2] / "queries")

router = APIRouter()
ADMIN_SCOPE_NAMES = {"admin"}


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


def _require_admin_scope(auth: dict, enums: Any) -> None:
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


def _append_scope(scope_ids: list[Any] | None, scope_id: Any) -> list[Any]:
    """Return scope list with scope_id appended only when missing."""

    current = list(scope_ids or [])
    if scope_id not in current:
        current.append(scope_id)
    return current


def _resolve_login_baseline(enums: Any) -> tuple[list[UUID], UUID, UUID]:
    """Return baseline scope/type/status IDs required for first-run login.

    Args:
        enums: Loaded enum registry from app state.

    Returns:
        Tuple of (baseline_scope_ids, person_type_id, active_status_id).
    """

    try:
        return (
            require_scopes(["public", "private", "sensitive", "admin"], enums),
            require_entity_type("person", enums),
            require_status("active", enums),
        )
    except ValueError as exc:
        api_error(
            "SERVICE_UNAVAILABLE",
            (
                "Login unavailable: required baseline taxonomy is missing "
                f"({exc}). Run migrations/seed and retry."
            ),
            503,
        )


class LoginInput(BaseModel):
    """Login payload used to create or fetch a user entity.

    Attributes:
        username: Human-readable username for the entity.
    """

    username: str


class CreateKeyInput(BaseModel):
    """Payload for creating a named API key.

    Attributes:
        name: Friendly name for the API key.
    """

    name: str


@router.post("/login")
async def login(payload: LoginInput, request: Request) -> dict[str, Any]:
    """First-run login: find or create user entity, generate API key.

    Args:
        payload: Login payload.
        request: FastAPI request.

    Returns:
        API response with generated API key and entity info.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    baseline_scope_ids, person_type_id, status_id = _resolve_login_baseline(enums)

    # Find existing entity by name + type person
    entity = await pool.fetchrow(
        QUERIES["entities/find_by_name_type"],
        payload.username,
        person_type_id,
    )

    if not entity:
        entity = await pool.fetchrow(
            QUERIES["entities/create"],
            baseline_scope_ids,
            payload.username,
            person_type_id,
            status_id,
            [],
            "{}",
            None,
        )
    else:
        scope_ids = list(entity.get("privacy_scope_ids") or [])
        for scope_id in baseline_scope_ids:
            scope_ids = _append_scope(scope_ids, scope_id)
        if scope_ids != (entity.get("privacy_scope_ids") or []):
            entity = await pool.fetchrow(
                QUERIES["entities/update_scope_ids"],
                entity["id"],
                scope_ids,
            )

    entity_id = entity["id"]
    raw_key, prefix, key_hash = generate_api_key()

    await pool.execute(
        QUERIES["api_keys/create_for_entity"],
        entity_id,
        key_hash,
        prefix,
        "default",
    )

    return success(
        {
            "api_key": raw_key,
            "entity_id": str(entity_id),
            "username": payload.username,
        }
    )


@router.post("/")
async def create_key(
    payload: CreateKeyInput,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Generate a new API key for the authenticated user.

    Args:
        payload: API key creation payload.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with newly created API key.
    """

    pool = request.app.state.pool

    raw_key, prefix, key_hash = generate_api_key()

    row = await pool.fetchrow(
        QUERIES["api_keys/create_with_returning"],
        auth["entity_id"],
        key_hash,
        prefix,
        payload.name,
    )

    return success(
        {
            "api_key": raw_key,
            "key_id": str(row["id"]),
            "prefix": row["key_prefix"],
            "name": row["name"],
        }
    )


@router.get("/all")
async def list_all_keys(
    request: Request, auth: dict = Depends(require_auth)
) -> dict[str, Any]:
    """List all active API keys with owner info.

    Args:
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with API key list.
    """

    pool = request.app.state.pool
    enums = request.app.state.enums
    _require_admin_scope(auth, enums)

    rows = await pool.fetch(QUERIES["api_keys/list_all"], 5000, 0)
    return success([dict(r) for r in rows])


@router.get("/")
async def list_keys(
    request: Request, auth: dict = Depends(require_auth)
) -> dict[str, Any]:
    """List all active API keys for the authenticated user.

    Args:
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with API key list.
    """

    pool = request.app.state.pool

    rows = await pool.fetch(QUERIES["api_keys/list_by_entity"], auth["entity_id"])

    return success([dict(r) for r in rows])


@router.delete("/{key_id}")
async def revoke_key(
    key_id: str,
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Revoke an API key.

    Args:
        key_id: API key id.
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with revocation result.
    """

    pool = request.app.state.pool
    _require_uuid(key_id, "key")

    result = await pool.execute(
        QUERIES["api_keys/revoke"],
        key_id,
        auth["entity_id"],
    )

    if result == "UPDATE 0":
        api_error("NOT_FOUND", "Key not found or already revoked", 404)

    return success({"revoked": True})
