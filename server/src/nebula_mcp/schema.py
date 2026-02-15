"""Schema contract loader for agents and clients.

This module exposes a canonical "schema" view of active taxonomy and core enum
constraints. Agents should query this before inventing scope/type/status values.
"""

# Standard Library
from pathlib import Path
from typing import Any

# Third-Party
from asyncpg import Pool

# Local
from .query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[1] / "queries")

JOB_PRIORITY_VALUES = ["low", "medium", "high", "critical"]
APPROVAL_STATUS_VALUES = ["pending", "approved", "rejected", "approved-failed"]
RELATIONSHIP_NODE_TYPE_VALUES = [
    "entity",
    "knowledge",
    "log",
    "job",
    "agent",
    "file",
    "protocol",
]
AUDIT_ACTION_VALUES = ["insert", "update", "delete"]
AUDIT_ACTOR_TYPE_VALUES = ["agent", "entity", "system"]


def _stringify_ids(
    rows: list[dict[str, Any]],
    *,
    id_key: str = "id",
) -> list[dict[str, Any]]:
    """Convert UUID ids to strings for JSON-safe tool responses."""

    out: list[dict[str, Any]] = []
    for row in rows:
        copy = dict(row)
        if id_key in copy and copy[id_key] is not None:
            copy[id_key] = str(copy[id_key])
        out.append(copy)
    return out


async def load_schema_contract(pool: Pool) -> dict[str, Any]:
    """Return the canonical schema contract for active taxonomy + constraints.

    Args:
        pool: Database connection pool.

    Returns:
        Dict containing active taxonomy lists, statuses, and core constraints.
    """

    scopes = _stringify_ids(
        [dict(r) for r in await pool.fetch(QUERIES["schema/list_active_scopes"])]
    )
    entity_types = _stringify_ids(
        [dict(r) for r in await pool.fetch(QUERIES["schema/list_active_entity_types"])]
    )
    relationship_types = _stringify_ids(
        [
            dict(r)
            for r in await pool.fetch(QUERIES["schema/list_active_relationship_types"])
        ]
    )
    log_types = _stringify_ids(
        [dict(r) for r in await pool.fetch(QUERIES["schema/list_active_log_types"])]
    )
    statuses = _stringify_ids(
        [dict(r) for r in await pool.fetch(QUERIES["schema/list_statuses"])]
    )

    return {
        "taxonomy": {
            "scopes": scopes,
            "entity_types": entity_types,
            "relationship_types": relationship_types,
            "log_types": log_types,
        },
        "statuses": statuses,
        "constraints": {
            "jobs": {
                "priority": JOB_PRIORITY_VALUES,
            },
            "approval_requests": {
                "status": APPROVAL_STATUS_VALUES,
            },
            "relationships": {
                "node_types": RELATIONSHIP_NODE_TYPE_VALUES,
            },
            "audit_log": {
                "action": AUDIT_ACTION_VALUES,
                "actor_type": AUDIT_ACTOR_TYPE_VALUES,
            },
        },
    }
