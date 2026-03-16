"""Context extraction and validation helpers for MCP tools."""

import json
import os
from pathlib import Path
from typing import Any

from asyncpg import Pool
from mcp.server.fastmcp import Context

from .db import get_agent
from .enums import EnumRegistry
from .query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[1] / "queries")

# --- Type Aliases ---

AgentDict = dict[str, Any]

LOCAL_INSECURE_MODE_ENV = "NEBULA_MCP_LOCAL_INSECURE"
LOCAL_INSECURE_AGENT_ENV = "NEBULA_MCP_LOCAL_AGENT_NAME"
LOCAL_INSECURE_DEFAULT_AGENT = "local-dev-agent"
LOCAL_INSECURE_SCOPE_ORDER = ("public", "private", "sensitive", "admin")


def enrollment_required_error() -> ValueError:
    """Build a structured error for unauthenticated bootstrap callers."""

    payload = {
        "error": {
            "code": "ENROLLMENT_REQUIRED",
            "message": "Agent not enrolled",
            "next_steps": [
                "agent_enroll_start",
                "agent_enroll_wait",
                "agent_enroll_redeem",
                "agent_auth_attach",
            ],
        }
    }
    return ValueError(json.dumps(payload))


async def require_context(
    ctx: Context, *, allow_bootstrap: bool = False
) -> tuple[Pool, EnumRegistry, AgentDict | None]:
    """Extract pool, enums, and agent from context or raise.

    Args:
        ctx: MCP request context.

    Returns:
        Tuple of (pool, enums, agent) from lifespan context.

    Raises:
        ValueError: If pool, enums, or agent not initialized.
    """

    lifespan_ctx = ctx.request_context.lifespan_context

    if not lifespan_ctx or "pool" not in lifespan_ctx:
        raise ValueError("Pool not initialized")

    if "enums" not in lifespan_ctx:
        raise ValueError("Enums not initialized")

    if "agent" not in lifespan_ctx:
        raise ValueError("Agent not initialized")

    agent = lifespan_ctx.get("agent")
    if agent is not None:
        refreshed = await lifespan_ctx["pool"].fetchrow(
            QUERIES["agents/get_by_id"], str(agent["id"])
        )
        if not refreshed:
            raise ValueError("Agent not found or inactive")
        agent = dict(refreshed)
        lifespan_ctx["agent"] = agent

    if agent is None and not allow_bootstrap:
        raise enrollment_required_error()

    return lifespan_ctx["pool"], lifespan_ctx["enums"], agent


async def require_pool(ctx: Context) -> Pool:
    """Extract pool from context when enums not needed.

    Args:
        ctx: MCP request context.

    Returns:
        Database connection pool.

    Raises:
        ValueError: If pool not initialized.
    """

    lifespan_ctx = ctx.request_context.lifespan_context

    if not lifespan_ctx or "pool" not in lifespan_ctx:
        raise ValueError("Pool not initialized")

    return lifespan_ctx["pool"]


async def require_agent(pool: Pool, agent_name: str) -> AgentDict:
    """Validate agent exists and is active.

    Args:
        pool: Database connection pool.
        agent_name: Agent name to validate.

    Returns:
        Agent row with id, scopes, requires_approval, etc.

    Raises:
        ValueError: If agent not found or inactive.
    """

    agent = await get_agent(pool, agent_name)

    if not agent:
        raise ValueError("Agent not found or inactive")

    return agent


async def maybe_require_approval(
    pool: Pool,
    agent: AgentDict,
    action: str,
    payload_dict: dict,
) -> dict | None:
    """Return approval response if agent requires it, else None.

    Checks agent trust level and routes to approval workflow if needed.
    Trusted agents return None to proceed with direct execution.

    Args:
        pool: Database connection pool.
        agent: Agent dict with requires_approval field.
        action: Action name (e.g., create_entity).
        payload_dict: Full payload for the action.

    Returns:
        Approval response if untrusted, None if trusted.
    """

    # Import here to avoid circular dependency
    from .helpers import create_approval_request, ensure_approval_capacity

    if not agent.get("requires_approval", True):
        return None

    await ensure_approval_capacity(pool, agent["id"])

    approval = await create_approval_request(
        pool,
        agent["id"],
        action,
        payload_dict,
        None,
    )

    return {
        "status": "approval_required",
        "approval_request_id": str(approval["id"]) if approval else None,
        "message": "Approval request created. Waiting for review.",
        "requested_action": action,
    }


async def authenticate_agent(pool: Pool) -> AgentDict:
    """Authenticate MCP server agent via NEBULA_API_KEY env var.

    Reads the API key from environment, validates against DB, loads agent row.

    Args:
        pool: Database connection pool.

    Returns:
        Agent row with id, name, scopes, requires_approval, etc.

    Raises:
        ValueError: If key missing, invalid, or agent inactive.
    """

    api_key = os.environ.get("NEBULA_API_KEY")
    if not api_key:
        raise ValueError(
            "NEBULA_API_KEY environment variable is required for MCP server authentication"
        )

    return await authenticate_agent_with_key(pool, api_key, key_name="NEBULA_API_KEY")


async def authenticate_agent_with_key(
    pool: Pool, api_key: str, *, key_name: str = "API key"
) -> AgentDict:
    """Authenticate an agent using a raw API key value."""

    from argon2 import PasswordHasher
    from argon2.exceptions import VerifyMismatchError

    ph = PasswordHasher()

    if len(api_key) < 8:
        raise ValueError(f"{key_name} is too short")

    prefix = api_key[:8]
    row = await pool.fetchrow(QUERIES["api_keys/get_by_prefix"], prefix)
    if not row:
        raise ValueError(f"{key_name} is invalid or revoked")

    try:
        ph.verify(row["key_hash"], api_key)
    except VerifyMismatchError:
        raise ValueError(f"{key_name} hash mismatch")

    if not row["agent_id"]:
        raise ValueError(f"{key_name} is not an agent key")

    agent = await pool.fetchrow(QUERIES["agents/get_by_id"], row["agent_id"])
    if not agent:
        raise ValueError(f"Agent not found or inactive for {key_name}")

    return dict(agent)


def _env_truthy(name: str) -> bool:
    """Handle env truthy.

    Args:
        name: Input parameter for _env_truthy.

    Returns:
        Result value from the operation.
    """

    value = os.environ.get(name, "")
    return value.strip().lower() in {"1", "true", "yes", "on"}


def _local_insecure_agent_name() -> str:
    """Handle local insecure agent name.

    Returns:
        Result value from the operation.
    """

    value = os.environ.get(LOCAL_INSECURE_AGENT_ENV, "")
    cleaned = value.strip()
    if cleaned:
        return cleaned
    return LOCAL_INSECURE_DEFAULT_AGENT


async def _get_or_create_local_insecure_agent(pool: Pool, enums: EnumRegistry) -> AgentDict:
    """Handle get or create local insecure agent.

    Args:
        pool: Input parameter for _get_or_create_local_insecure_agent.
        enums: Input parameter for _get_or_create_local_insecure_agent.

    Returns:
        Result value from the operation.
    """

    name = _local_insecure_agent_name()
    existing = await pool.fetchrow(QUERIES["agents/get"], name)
    if existing:
        return dict(existing)

    scope_ids: list[Any] = []
    for scope_name in LOCAL_INSECURE_SCOPE_ORDER:
        scope_id = enums.scopes.name_to_id.get(scope_name)
        if scope_id:
            scope_ids.append(scope_id)
    if not scope_ids:
        raise ValueError("Local insecure mode requires at least one valid scope")

    status_id = enums.statuses.name_to_id.get("active")
    if not status_id:
        raise ValueError("Local insecure mode requires active status enum")

    created = await pool.fetchrow(
        QUERIES["agents/create"],
        name,
        "Local insecure MCP session agent",
        scope_ids,
        ["local-insecure"],
        status_id,
        False,
    )
    if not created:
        raise ValueError("Failed to create local insecure agent")
    return dict(created)


async def authenticate_agent_optional(
    pool: Pool, enums: EnumRegistry
) -> tuple[AgentDict | None, bool]:
    """Authenticate MCP server agent when key exists, or enter bootstrap mode.

    Returns:
        Tuple of (agent, bootstrap_mode).
    """

    api_key = os.environ.get("NEBULA_API_KEY")
    if not api_key:
        if _env_truthy(LOCAL_INSECURE_MODE_ENV):
            agent = await _get_or_create_local_insecure_agent(pool, enums)
            return agent, False
        return None, True

    agent = await authenticate_agent(pool)
    return agent, False
