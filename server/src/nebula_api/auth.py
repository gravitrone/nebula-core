"""API key authentication middleware."""

import secrets
from pathlib import Path
from typing import Any

from argon2 import PasswordHasher
from argon2.exceptions import VerifyMismatchError
from asyncpg import Pool
from fastapi import HTTPException, Request
from starlette.responses import JSONResponse

from nebula_mcp.query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[1] / "queries")

ph = PasswordHasher()


def _merge_scopes(key_scopes: list | None, owner_scopes: list | None) -> list:
    """Return owner scopes narrowed by key scopes when provided."""

    key_scopes = key_scopes or []
    owner_scopes = owner_scopes or []
    if key_scopes:
        owner_set = set(owner_scopes)
        return [s for s in key_scopes if s in owner_set]
    return list(owner_scopes)


def generate_api_key() -> tuple[str, str, str]:
    """Generate a new API key.

    Returns:
        Tuple of (raw_key, prefix, key_hash).
    """

    raw = "nbl_" + secrets.token_urlsafe(36)
    prefix = raw[:8]
    key_hash = ph.hash(raw)
    return raw, prefix, key_hash


async def require_auth(request: Request) -> dict:
    """Validate API key and return auth context.

    Args:
        request: FastAPI request object.

    Returns:
        Auth context dict with key_id, caller_type, entity/agent info.

    Raises:
        HTTPException: If API key is missing or invalid.
    """

    auth_header = request.headers.get("Authorization")
    if not auth_header or not auth_header.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="Missing API key")

    raw_key = auth_header.split(" ", 1)[1]
    if len(raw_key) < 8:
        raise HTTPException(status_code=401, detail="Invalid API key")

    prefix = raw_key[:8]
    pool = request.app.state.pool

    row = await pool.fetchrow(QUERIES["api_keys/get_by_prefix"], prefix)
    if not row:
        raise HTTPException(status_code=401, detail="Invalid API key")

    try:
        ph.verify(row["key_hash"], raw_key)
    except VerifyMismatchError:
        raise HTTPException(status_code=401, detail="Invalid API key")

    # Update last_used_at (fire-and-forget)
    await pool.execute(QUERIES["api_keys/update_last_used"], row["id"])

    # Branch on key owner type
    if row["entity_id"]:
        entity = await pool.fetchrow(QUERIES["entities/get_by_id"], row["entity_id"])
        entity_scopes = []
        if entity:
            entity_scopes = entity.get("privacy_scope_ids") or []
        scopes = _merge_scopes(row["scopes"], entity_scopes)
        return {
            "key_id": row["id"],
            "caller_type": "user",
            "entity_id": row["entity_id"],
            "entity": dict(entity) if entity else None,
            "agent_id": None,
            "agent": None,
            "scopes": scopes,
        }

    if row["agent_id"]:
        agent = await pool.fetchrow(QUERIES["agents/get_by_id"], row["agent_id"])
        if not agent:
            raise HTTPException(status_code=401, detail="Agent not found or inactive")
        scopes = _merge_scopes(row["scopes"], agent.get("scopes"))

        return {
            "key_id": row["id"],
            "caller_type": "agent",
            "entity_id": None,
            "entity": None,
            "agent_id": row["agent_id"],
            "agent": dict(agent),
            "scopes": scopes,
        }

    raise HTTPException(status_code=401, detail="Invalid API key")


async def maybe_check_agent_approval(
    pool: Pool,
    auth: dict[str, Any],
    action: str,
    payload: dict[str, Any],
) -> JSONResponse | None:
    """Check if caller is untrusted agent and create approval request.

    Args:
        pool: Database connection pool.
        auth: Auth context from require_auth.
        action: Action type (e.g., create_entity).
        payload: Action payload dict.

    Returns:
        JSONResponse with 202 status if approval required, None otherwise.
    """

    if auth["caller_type"] != "agent":
        return None
    agent = auth["agent"]
    if not agent.get("requires_approval", True):
        return None

    from nebula_mcp.helpers import create_approval_request, ensure_approval_capacity

    try:
        await ensure_approval_capacity(pool, agent["id"])
    except ValueError as exc:
        return JSONResponse(
            status_code=429,
            content={
                "status": "rate_limited",
                "message": str(exc),
            },
        )

    approval = await create_approval_request(pool, agent["id"], action, payload)
    return JSONResponse(
        status_code=202,
        content={
            "status": "approval_required",
            "approval_request_id": str(approval["id"]),
            "message": "Approval request created. Waiting for review.",
        },
    )
