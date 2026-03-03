"""Enum registry and validators for Nebula MCP."""

# Standard Library
from dataclasses import dataclass
from pathlib import Path
from uuid import UUID

# Third-Party
from asyncpg import Pool

# Local
from .query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[1] / "queries")


@dataclass
class EnumSection:
    """Bidirectional mapping between enum names and UUIDs."""

    name_to_id: dict[str, UUID]
    id_to_name: dict[UUID, str]


@dataclass
class EnumRegistry:
    """Container for all enum sections."""

    statuses: EnumSection
    scopes: EnumSection
    relationship_types: EnumSection
    entity_types: EnumSection
    log_types: EnumSection


async def _load_section(pool: Pool, query_name: str) -> EnumSection:
    """Load one enum section using a named SQL query."""

    rows = await pool.fetch(QUERIES[query_name])
    name_to_id = {r["name"]: r["id"] for r in rows}
    id_to_name = {r["id"]: r["name"] for r in rows}

    return EnumSection(name_to_id=name_to_id, id_to_name=id_to_name)


async def load_enums(pool: Pool) -> EnumRegistry:
    """Load all enum sections into a registry."""

    return EnumRegistry(
        statuses=await _load_section(pool, "enums/statuses"),
        scopes=await _load_section(pool, "enums/scopes"),
        relationship_types=await _load_section(pool, "enums/relationship_types"),
        entity_types=await _load_section(pool, "enums/entity_types"),
        log_types=await _load_section(pool, "enums/log_types"),
    )


def _require(name: str, mapping: dict[str, UUID], label: str) -> UUID:
    """Return an ID for a name or raise a clear error."""

    try:
        return mapping[name]
    except KeyError:
        raise ValueError(f"Unknown {label}: {name}")


def require_status(name: str, enums: EnumRegistry) -> UUID:
    """Validate and return a status ID from a name."""

    if not isinstance(name, str) or not name:
        raise ValueError("Status required")
    key = name.strip().lower()
    if key == "archived":
        for candidate in (
            "inactive",
            "completed",
            "abandoned",
            "deleted",
            "replaced",
            "on-hold",
        ):
            if candidate in enums.statuses.name_to_id:
                key = candidate
                break
    return _require(key, enums.statuses.name_to_id, "status")


def require_entity_type(name: str, enums: EnumRegistry) -> UUID:
    """Validate and return an entity type ID from a name."""

    if not name:
        raise ValueError("Entity type required")
    return _require(name, enums.entity_types.name_to_id, "entity type")


def require_relationship_type(name: str, enums: EnumRegistry) -> UUID:
    """Validate and return a relationship type ID from a name."""

    if not name:
        raise ValueError("Relationship type required")
    return _require(name, enums.relationship_types.name_to_id, "relationship type")


def require_scopes(names: list[str], enums: EnumRegistry) -> list[UUID]:
    """Validate and return scope IDs from names."""

    if not names:
        raise ValueError("Scopes required")
    ids: list[UUID] = []
    for n in names:
        ids.append(_require(n, enums.scopes.name_to_id, "scope"))
    return ids


def require_log_type(name: str, enums: EnumRegistry) -> UUID:
    """Validate and return a log type ID from a name."""

    if not name:
        raise ValueError("Log type required")
    return _require(name, enums.log_types.name_to_id, "log type")
