"""Pure helper functions for Nebula MCP."""

# Standard Library
import inspect
import json
import re
import secrets
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any
from uuid import UUID

# Third-Party
from argon2 import PasswordHasher
from argon2.exceptions import VerifyMismatchError
from asyncpg import Connection, Pool

# Local
from .enums import EnumRegistry, require_scopes
from .query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[1] / "queries")
ph = PasswordHasher()


def scope_names_from_ids(scope_ids: list, enums: EnumRegistry) -> list[str]:
    """Resolve scope UUIDs to names."""

    names: list[str] = []
    for scope_id in scope_ids or []:
        name = enums.scopes.id_to_name.get(scope_id)
        if name:
            names.append(name)
    return names


def enforce_scope_subset(requested: list[str], allowed: list[str]) -> list[str]:
    """Ensure requested scopes are within allowed scope names."""

    if not requested:
        return list(allowed)
    requested_set = set(requested)
    allowed_set = set(allowed)
    if not requested_set.issubset(allowed_set):
        missing = sorted(requested_set - allowed_set)
        raise ValueError(
            f"Requested scopes exceed allowed scopes: {', '.join(missing)}"
        )
    return list(requested)


# --- Privacy Filtering ---


def filter_context_segments(metadata: dict | str, agent_scopes: list[str]) -> dict:
    """Filter context segments by agent's privacy scopes.

    Args:
        metadata: Entity metadata dict or JSON string.
        agent_scopes: List of scope names the agent has access to.

    Returns:
        Metadata dict with context_segments filtered to matching scopes.
    """

    # Handle JSONB string from asyncpg
    if isinstance(metadata, str):
        metadata = json.loads(metadata)

    if not metadata or "context_segments" not in metadata:
        return metadata

    filtered = metadata.copy()
    segments = []

    for seg in metadata["context_segments"]:
        seg_scopes = seg.get("scopes", [])
        if any(scope in agent_scopes for scope in seg_scopes):
            segments.append(seg)

    filtered["context_segments"] = segments
    return filtered


# --- Approval Workflow ---

MAX_PENDING_APPROVALS = 10
ENROLLMENT_TTL_HOURS = 24
ENROLLMENT_WAIT_POLL_SECONDS = 1


def generate_enrollment_token() -> tuple[str, str]:
    """Generate raw bootstrap enrollment token and hashed storage value."""

    raw = "nbe_" + secrets.token_urlsafe(36)
    return raw, ph.hash(raw)


def verify_enrollment_token(token_hash: str, raw_token: str) -> bool:
    """Return True when enrollment token matches the stored hash."""

    try:
        return ph.verify(token_hash, raw_token)
    except VerifyMismatchError:
        return False


async def ensure_approval_capacity(
    pool: Pool,
    agent_id: str,
    requested: int = 1,
    conn: Any | None = None,
) -> None:
    """Raise if agent has too many pending approvals."""

    if conn is None:
        acquire = getattr(pool, "acquire", None)
        if acquire is None or inspect.iscoroutinefunction(acquire):
            fetchval = getattr(pool, "fetchval", None)
            if fetchval is None:
                return
            count = await fetchval(
                (
                    "SELECT COUNT(*) FROM approval_requests "
                    "WHERE status = 'pending' AND requested_by = $1"
                ),
                agent_id,
            )
            if count is None:
                return
            try:
                count_value = int(count)
            except (TypeError, ValueError):
                return
            if requested < 1:
                return
            if count_value + requested > MAX_PENDING_APPROVALS:
                raise ValueError("Approval queue limit reached")
            return
        async with acquire() as pooled:
            async with pooled.transaction():
                await ensure_approval_capacity(
                    pool, agent_id, requested=requested, conn=pooled
                )
        return

    await conn.execute(
        "SELECT pg_advisory_xact_lock(hashtext($1))",
        str(agent_id),
    )
    count = await conn.fetchval(
        (
            "SELECT COUNT(*) FROM approval_requests "
            "WHERE status = 'pending' AND requested_by = $1"
        ),
        agent_id,
    )
    if count is None:
        return
    try:
        count_value = int(count)
    except (TypeError, ValueError):
        return
    if requested < 1:
        return
    if count_value + requested > MAX_PENDING_APPROVALS:
        raise ValueError("Approval queue limit reached")


async def create_approval_request(
    pool: Pool,
    agent_id: str,
    request_type: str,
    change_details: dict,
    job_id: str | None = None,
    conn: Connection | None = None,
) -> dict:
    """Create an approval request for untrusted agent actions.

    Args:
        pool: Database connection pool.
        agent_id: UUID of the requesting agent.
        request_type: Action type (e.g., create_entity).
        change_details: Full payload of requested change.
        job_id: Optional related job ID.

    Returns:
        Created approval request row as dict.
    """

    async def _create(active_conn: Any) -> dict:
        await ensure_approval_capacity(pool, agent_id, conn=active_conn)
        row = await active_conn.fetchrow(
            QUERIES["approvals/create_request"],
            request_type,
            agent_id,
            (
                json.dumps(change_details)
                if isinstance(change_details, dict)
                else change_details
            ),
            job_id,
        )
        return dict(row) if row else {}

    if conn is not None:
        return await _create(conn)

    async with pool.acquire() as acquired:
        async with acquired.transaction():
            return await _create(acquired)


async def create_enrollment_session(
    pool: Pool,
    *,
    agent_id: str,
    approval_request_id: str,
    requested_scope_ids: list[str],
    requested_requires_approval: bool,
    conn: Connection | None = None,
) -> dict:
    """Create and persist a new enrollment session linked to an approval request."""

    raw_token, token_hash = generate_enrollment_token()
    expires_at = datetime.now(UTC) + timedelta(hours=ENROLLMENT_TTL_HOURS)
    fetcher = conn.fetchrow if conn is not None else pool.fetchrow
    row = await fetcher(
        QUERIES["enrollments/create"],
        agent_id,
        approval_request_id,
        token_hash,
        requested_scope_ids,
        requested_requires_approval,
        expires_at,
    )
    if not row:
        raise ValueError("Failed to create enrollment session")
    payload = dict(row)
    payload["enrollment_token"] = raw_token
    return payload


async def get_enrollment_for_wait(
    pool: Pool,
    *,
    registration_id: str,
    enrollment_token: str,
) -> dict:
    """Load enrollment session and validate enrollment token."""

    row = await pool.fetchrow(QUERIES["enrollments/get_by_id"], registration_id)
    if not row:
        raise ValueError("Enrollment not found")
    data = dict(row)
    if not verify_enrollment_token(data["enrollment_token_hash"], enrollment_token):
        raise ValueError("Invalid enrollment token")
    return data


async def maybe_expire_enrollment(pool: Pool, registration_id: str) -> dict | None:
    """Expire pending/approved enrollment when TTL elapsed."""

    row = await pool.fetchrow(QUERIES["enrollments/get_by_id"], registration_id)
    if not row:
        return None
    data = dict(row)
    expires_at = data.get("expires_at")
    if (
        expires_at
        and datetime.now(UTC) >= expires_at
        and data.get("status")
        in {
            "pending_approval",
            "approved",
        }
    ):
        updated = await pool.fetchrow(
            QUERIES["enrollments/mark_expired"], registration_id
        )
        if updated:
            return dict(updated)
    return data


async def wait_for_enrollment_status(
    pool: Pool,
    *,
    registration_id: str,
    enrollment_token: str,
    timeout_seconds: int,
) -> dict:
    """Long-poll enrollment status for bounded wait windows."""

    started = datetime.now(UTC)
    timeout_seconds = max(1, min(timeout_seconds, 60))

    while True:
        session = await get_enrollment_for_wait(
            pool,
            registration_id=registration_id,
            enrollment_token=enrollment_token,
        )
        maybe_expired = await maybe_expire_enrollment(pool, registration_id)
        if maybe_expired:
            session = maybe_expired

        status = session.get("status")
        if status in {"approved", "rejected", "redeemed", "expired"}:
            return session

        elapsed = (datetime.now(UTC) - started).total_seconds()
        if elapsed >= timeout_seconds:
            session["retry_after_ms"] = ENROLLMENT_WAIT_POLL_SECONDS * 1000
            return session

        await pool.execute("SELECT pg_sleep($1)", ENROLLMENT_WAIT_POLL_SECONDS)


async def redeem_enrollment_key(
    pool: Pool,
    *,
    registration_id: str,
    enrollment_token: str,
) -> dict:
    """Redeem approved enrollment exactly once and mint an agent API key."""

    from nebula_api.auth import generate_api_key

    async with pool.acquire() as conn:
        async with conn.transaction():
            row = await conn.fetchrow(
                QUERIES["enrollments/get_for_update"], registration_id
            )
            if not row:
                raise ValueError("Enrollment not found")

            session = dict(row)
            if not verify_enrollment_token(
                session["enrollment_token_hash"],
                enrollment_token,
            ):
                raise ValueError("Invalid enrollment token")

            if session.get("expires_at") and datetime.now(UTC) >= session["expires_at"]:
                await conn.execute(QUERIES["enrollments/mark_expired"], registration_id)
                raise ValueError("Enrollment expired")

            status = session.get("status")
            if status == "redeemed":
                raise ValueError("Enrollment already redeemed")
            if status != "approved":
                raise ValueError("Enrollment is not approved")

            raw_key, prefix, key_hash = generate_api_key()
            await conn.execute(
                QUERIES["api_keys/create"],
                session["agent_id"],
                key_hash,
                prefix,
                f"agent-{session['agent_name']}",
            )

            marked = await conn.fetchrow(
                QUERIES["enrollments/mark_redeemed"], registration_id
            )
            if not marked:
                raise ValueError("Enrollment redemption failed")

            return {
                "api_key": raw_key,
                "agent_id": str(session["agent_id"]),
                "agent_name": session["agent_name"],
                "scope_ids": session.get("granted_scope_ids")
                or session.get("requested_scope_ids")
                or [],
                "requires_approval": (
                    session.get("granted_requires_approval")
                    if session.get("granted_requires_approval") is not None
                    else session.get("requested_requires_approval", True)
                ),
            }


async def get_pending_approvals_all(
    pool: Pool, limit: int = 200, offset: int = 0
) -> list[dict]:
    """Get all pending approval requests for admin review.

    Args:
        pool: Database connection pool.
        limit: Max rows to return.
        offset: Pagination offset.

    Returns:
        List of pending approval request dicts.
    """

    rows = await pool.fetch(QUERIES["approvals/get_pending"], limit, offset)
    return await _enrich_approval_rows(pool, [dict(r) for r in rows])


async def get_approval_request(pool: Pool, approval_id: str) -> dict | None:
    """Get one approval request with human-readable enrichment."""

    row = await pool.fetchrow(QUERIES["approvals/get_request"], approval_id)
    if not row:
        return None
    enriched = await _enrich_approval_rows(pool, [dict(row)])
    return enriched[0] if enriched else None


def _safe_parse_uuid(value: str | None) -> str | None:
    if not value:
        return None
    text = str(value).strip()
    if not text:
        return None
    try:
        return str(UUID(text))
    except ValueError:
        return None


def _to_uuid_list(values: set[str]) -> list[str]:
    out: list[str] = []
    for value in values:
        parsed = _safe_parse_uuid(value)
        if parsed:
            out.append(parsed)
    return out


def _normalize_change_details(raw: Any) -> dict[str, Any]:
    if isinstance(raw, dict):
        return dict(raw)
    if isinstance(raw, str):
        try:
            parsed = json.loads(raw)
        except json.JSONDecodeError:
            return {}
        if isinstance(parsed, dict):
            return parsed
    return {}


def _extract_string_list(value: Any) -> list[str]:
    if isinstance(value, list):
        return [str(v).strip() for v in value if str(v).strip()]
    if isinstance(value, tuple | set):
        return [str(v).strip() for v in value if str(v).strip()]
    if isinstance(value, str):
        text = value.strip()
        if not text:
            return []
        if text.startswith("{") and text.endswith("}"):
            inner = text[1:-1].strip()
            if not inner:
                return []
            return [
                part.strip().strip('"').strip("'")
                for part in inner.split(",")
                if part.strip().strip('"').strip("'")
            ]
        if text.startswith("[") and text.endswith("]"):
            try:
                parsed = json.loads(text)
            except json.JSONDecodeError:
                parsed = None
            if isinstance(parsed, list):
                return [str(v).strip() for v in parsed if str(v).strip()]
        return [part.strip() for part in text.split(",") if part.strip()]
    return []


async def _fetch_name_map(
    pool: Pool, sql: str, ids: set[str]
) -> dict[str, str]:
    uuid_ids = _to_uuid_list(ids)
    if not uuid_ids:
        return {}
    rows = await pool.fetch(sql, uuid_ids)
    resolved: dict[str, str] = {}
    for row in rows:
        identifier = str(row["id"])
        label = str(row["label"]).strip() if row["label"] else ""
        if identifier and label:
            resolved[identifier] = label
    return resolved


async def _fetch_name_map_text(
    pool: Pool, sql: str, ids: set[str]
) -> dict[str, str]:
    text_ids = sorted({str(v).strip() for v in ids if str(v).strip()})
    if not text_ids:
        return {}
    rows = await pool.fetch(sql, text_ids)
    resolved: dict[str, str] = {}
    for row in rows:
        identifier = str(row["id"]).strip()
        label = str(row["label"]).strip() if row["label"] else ""
        if identifier and label:
            resolved[identifier] = label
    return resolved


async def _enrich_approval_rows(pool: Pool, rows: list[dict]) -> list[dict]:
    if not rows:
        return rows

    requested_ids: set[str] = set()
    entity_ids: set[str] = set()
    context_ids: set[str] = set()
    job_ids: set[str] = set()
    log_ids: set[str] = set()
    file_ids: set[str] = set()
    protocol_ids: set[str] = set()
    agent_ids: set[str] = set()
    relationship_ids: set[str] = set()

    for row in rows:
        requested = _safe_parse_uuid(row.get("requested_by"))
        if requested:
            requested_ids.add(requested)

        details = _normalize_change_details(row.get("change_details"))
        row["change_details"] = details
        relationship_id = _safe_parse_uuid(details.get("relationship_id"))
        if relationship_id:
            relationship_ids.add(relationship_id)

        for key in ("entity_id", "source_id", "target_id"):
            raw_value = details.get(key)
            value_text = str(raw_value).strip() if raw_value is not None else ""
            node_type = details.get(key.replace("_id", "_type"))
            if key == "entity_id":
                parsed = _safe_parse_uuid(value_text)
                if parsed:
                    entity_ids.add(parsed)
                continue
            if not isinstance(node_type, str) or not value_text:
                continue
            node_type = node_type.strip().lower()
            if node_type == "job":
                job_ids.add(value_text)
                continue
            parsed = _safe_parse_uuid(value_text)
            if not parsed:
                continue
            if node_type == "entity":
                entity_ids.add(parsed)
            elif node_type == "context":
                context_ids.add(parsed)
            elif node_type == "log":
                log_ids.add(parsed)
            elif node_type == "file":
                file_ids.add(parsed)
            elif node_type == "protocol":
                protocol_ids.add(parsed)
            elif node_type == "agent":
                agent_ids.add(parsed)

        for entity_id in _extract_string_list(details.get("entity_ids")):
            parsed = _safe_parse_uuid(entity_id)
            if parsed:
                entity_ids.add(parsed)

    entity_name_map = await _fetch_name_map(
        pool,
        "SELECT id::text AS id, name AS label FROM entities WHERE id = ANY($1::uuid[])",
        entity_ids,
    )
    context_name_map = await _fetch_name_map(
        pool,
        (
            "SELECT id::text AS id, title AS label "
            "FROM context_items WHERE id = ANY($1::uuid[])"
        ),
        context_ids,
    )
    job_name_map = await _fetch_name_map_text(
        pool,
        "SELECT id::text AS id, title AS label FROM jobs WHERE id = ANY($1::text[])",
        job_ids,
    )
    log_name_map = await _fetch_name_map(
        pool,
        (
            "SELECT l.id::text AS id, COALESCE(lt.name, 'log') AS label "
            "FROM logs l LEFT JOIN log_types lt ON l.type_id = lt.id "
            "WHERE l.id = ANY($1::uuid[])"
        ),
        log_ids,
    )
    file_name_map = await _fetch_name_map(
        pool,
        (
            "SELECT id::text AS id, filename AS label "
            "FROM files WHERE id = ANY($1::uuid[])"
        ),
        file_ids,
    )
    protocol_name_map = await _fetch_name_map(
        pool,
        (
            "SELECT id::text AS id, COALESCE(title, name, 'protocol') AS label "
            "FROM protocols WHERE id = ANY($1::uuid[])"
        ),
        protocol_ids,
    )
    agent_name_map = await _fetch_name_map(
        pool,
        "SELECT id::text AS id, name AS label FROM agents WHERE id = ANY($1::uuid[])",
        requested_ids.union(agent_ids),
    )
    requested_entity_map = await _fetch_name_map(
        pool,
        "SELECT id::text AS id, name AS label FROM entities WHERE id = ANY($1::uuid[])",
        requested_ids,
    )
    relationship_map: dict[str, dict[str, Any]] = {}
    relationship_uuid_ids = _to_uuid_list(relationship_ids)
    if relationship_uuid_ids:
        rel_rows = await pool.fetch(
            """
            SELECT
                r.id::text AS id,
                rt.name AS relationship_type,
                r.source_type,
                r.source_id::text AS source_id,
                r.target_type,
                r.target_id::text AS target_id
            FROM relationships r
            LEFT JOIN relationship_types rt ON r.type_id = rt.id
            WHERE r.id = ANY($1::uuid[])
            """,
            relationship_uuid_ids,
        )
        relationship_map = {str(r["id"]): dict(r) for r in rel_rows}

    def resolve_node_label(node_type: str | None, node_id: str | None) -> str | None:
        kind = (node_type or "").strip().lower()
        if kind == "job":
            text_id = str(node_id or "").strip()
            if not text_id:
                return None
            return job_name_map.get(text_id)

        parsed_id = _safe_parse_uuid(node_id)
        if not parsed_id:
            return None
        if kind == "entity":
            return entity_name_map.get(parsed_id)
        if kind == "context":
            return context_name_map.get(parsed_id)
        if kind == "log":
            return log_name_map.get(parsed_id)
        if kind == "file":
            return file_name_map.get(parsed_id)
        if kind == "protocol":
            return protocol_name_map.get(parsed_id)
        if kind == "agent":
            return agent_name_map.get(parsed_id)
        return None

    for row in rows:
        requested_id = _safe_parse_uuid(row.get("requested_by"))
        row["requested_by_name"] = None
        if requested_id:
            row["requested_by_name"] = (
                agent_name_map.get(requested_id)
                or requested_entity_map.get(requested_id)
            )

        details = _normalize_change_details(row.get("change_details"))

        relationship_id = _safe_parse_uuid(details.get("relationship_id"))
        if relationship_id and relationship_id in relationship_map:
            rel = relationship_map[relationship_id]
            if not details.get("relationship_type") and rel.get("relationship_type"):
                details["relationship_type"] = rel["relationship_type"]
            if not details.get("source_type") and rel.get("source_type"):
                details["source_type"] = rel["source_type"]
            if not details.get("source_id") and rel.get("source_id"):
                details["source_id"] = rel["source_id"]
            if not details.get("target_type") and rel.get("target_type"):
                details["target_type"] = rel["target_type"]
            if not details.get("target_id") and rel.get("target_id"):
                details["target_id"] = rel["target_id"]
        source_label = resolve_node_label(
            details.get("source_type"), details.get("source_id")
        )
        target_label = resolve_node_label(
            details.get("target_type"), details.get("target_id")
        )
        if source_label:
            details["source_name"] = source_label
        if target_label:
            details["target_name"] = target_label

        entity_names: list[str] = []
        for raw_id in _extract_string_list(details.get("entity_ids")):
            parsed = _safe_parse_uuid(raw_id)
            if not parsed:
                continue
            name = entity_name_map.get(parsed)
            if name:
                entity_names.append(name)
        if entity_names:
            details["entity_names"] = entity_names
            if len(entity_names) == 1:
                details["entity_name"] = entity_names[0]

        entity_name = resolve_node_label("entity", details.get("entity_id"))
        if entity_name:
            details["entity_name"] = entity_name

        row["change_details"] = details

    return rows


async def approve_request(
    pool: Pool,
    enums: EnumRegistry,
    approval_id: str,
    reviewed_by: str | None,
    review_details: dict | None = None,
    review_notes: str | None = None,
) -> dict:
    """Approve request and execute the action.

    Args:
        pool: Database connection pool.
        enums: Enum registry for validation.
        approval_id: UUID of approval request.
        reviewed_by: UUID of approving entity, or None for MCP admin calls.

    Returns:
        Dict containing approval record and created entity.

    Raises:
        ValueError: If approval not found or no executor available.
    """

    from .executors import EXECUTORS

    approval = await pool.fetchrow(
        QUERIES["approvals/approve"],
        approval_id,
        reviewed_by,
        (
            json.dumps(review_details, default=str)
            if isinstance(review_details, dict)
            else json.dumps({})
        ),
        review_notes,
    )

    if not approval:
        raise ValueError("Approval request not found or already processed")

    request_type = str(approval["request_type"] or "").strip()
    normalized_request_type = re.sub(
        r"[^a-z0-9]+",
        "_",
        request_type.lower(),
    ).strip("_")
    executor = EXECUTORS.get(request_type) or EXECUTORS.get(normalized_request_type)
    if not executor:
        await pool.execute(
            QUERIES["approvals/mark_failed"],
            f"No executor for: {request_type}",
            approval_id,
        )
        raise ValueError(f"No executor for: {request_type}")

    try:
        if reviewed_by:
            await pool.execute("SET app.changed_by_type = 'entity'")
            await pool.fetchval(
                "SELECT set_config('app.changed_by_id', $1, false)", reviewed_by
            )
        else:
            await pool.execute("SET app.changed_by_type = 'system'")
            await pool.execute("RESET app.changed_by_id")

        if normalized_request_type == "register_agent":
            raw_review_details = approval.get("review_details") or {}
            if isinstance(raw_review_details, str):
                try:
                    raw_review_details = json.loads(raw_review_details)
                except json.JSONDecodeError:
                    raw_review_details = {}
            if not isinstance(raw_review_details, dict):
                raw_review_details = {}
            exec_review_details = dict(raw_review_details)
            exec_review_details["_approval_id"] = str(approval["id"])
            if reviewed_by:
                exec_review_details["_reviewed_by"] = str(reviewed_by)
            result = await executor(
                pool,
                enums,
                approval["change_details"],
                exec_review_details,
            )
        else:
            result = await executor(pool, enums, approval["change_details"])

        linked_id = None
        if isinstance(result, dict):
            candidate = result.get("id")
            if candidate is not None:
                linked_id = str(candidate)
        if linked_id:
            await pool.execute(
                QUERIES["approvals/link_audit"],
                str(approval_id),
                linked_id,
            )

        return {"approval": dict(approval), "entity": result}

    except Exception as e:
        await pool.execute(QUERIES["approvals/mark_failed"], str(e), approval_id)
        raise

    finally:
        await pool.execute("RESET app.changed_by_type")
        await pool.execute("RESET app.changed_by_id")


async def reject_request(
    pool: Pool, approval_id: str, reviewed_by: str | None, review_notes: str
) -> dict:
    """Reject an approval request.

    Args:
        pool: Database connection pool.
        approval_id: UUID of approval request.
        reviewed_by: UUID of rejecting entity, or None for MCP admin calls.
        review_notes: Reason for rejection.

    Returns:
        Rejected approval request row as dict.

    Raises:
        ValueError: If approval not found.
    """

    row = await pool.fetchrow(
        QUERIES["approvals/reject"],
        approval_id,
        reviewed_by,
        review_notes,
    )

    if not row:
        raise ValueError("Approval request not found or already processed")

    approval = dict(row)
    if approval.get("request_type") == "register_agent":
        await pool.execute(
            QUERIES["enrollments/mark_rejected"],
            approval_id,
            review_notes,
            reviewed_by,
        )

    return approval


# --- Audit + History ---


async def get_entity_history(
    pool: Pool, entity_id: str, limit: int = 50, offset: int = 0
) -> list[dict]:
    """List audit history entries for a single entity.

    Args:
        pool: Database connection pool.
        entity_id: Entity UUID.
        limit: Max rows to return.
        offset: Pagination offset.

    Returns:
        List of audit entries as dicts.
    """

    rows = await pool.fetch(QUERIES["audit/entity_history"], entity_id, limit, offset)
    return [dict(r) for r in rows]


async def revert_entity(pool: Pool, entity_id: str, audit_id: str) -> dict:
    """Revert an entity to a historical audit snapshot.

    Args:
        pool: Database connection pool.
        entity_id: Entity UUID to revert.
        audit_id: Audit log entry to restore.

    Returns:
        Updated entity row as dict.

    Raises:
        ValueError: If audit entry is missing or mismatched.
    """

    audit_row = await pool.fetchrow(QUERIES["audit/get"], audit_id)
    if not audit_row:
        raise ValueError("Audit entry not found")

    audit = dict(audit_row)
    if audit.get("table_name") != "entities":
        raise ValueError("Audit entry is not for entities")
    if audit.get("record_id") != entity_id:
        raise ValueError("Audit entry does not match entity")

    snapshot = audit.get("new_data")
    if audit.get("action") == "delete":
        snapshot = audit.get("old_data")
    if snapshot is None:
        raise ValueError("Audit snapshot is empty")

    if isinstance(snapshot, str):
        snapshot = json.loads(snapshot)

    metadata = snapshot.get("metadata")
    metadata_json = None
    if metadata is not None:
        metadata_json = json.dumps(metadata)

    row = await pool.fetchrow(
        QUERIES["entities/revert"],
        entity_id,
        snapshot.get("privacy_scope_ids") or [],
        snapshot.get("name"),
        snapshot.get("type_id"),
        snapshot.get("status_id"),
        snapshot.get("status_changed_at"),
        snapshot.get("status_reason"),
        snapshot.get("tags") or [],
        metadata_json,
        snapshot.get("source_path"),
    )
    return dict(row) if row else {}


def normalize_bulk_operation(op: str) -> str:
    """Normalize bulk operation name."""

    key = (op or "").strip().lower()
    if key in {"add", "+"}:
        return "add"
    if key in {"remove", "rm", "del", "delete", "-"}:
        return "remove"
    if key in {"set", "="}:
        return "set"
    raise ValueError("Invalid bulk operation. Use add, remove, or set.")


async def bulk_update_entity_tags(
    pool: Pool, entity_ids: list[str], tags: list[str], op: str
) -> list[str]:
    """Bulk update entity tags.

    Args:
        pool: Database connection pool.
        entity_ids: Entity UUIDs.
        tags: Tag values.
        op: add, remove, or set.

    Returns:
        Updated entity ids.
    """

    rows = await pool.fetch(QUERIES["entities/bulk_update_tags"], entity_ids, op, tags)
    updated_ids: list[str] = []
    for row in rows:
        row_id = row.get("id")
        if row_id is None and len(row.values()) > 0:
            row_id = list(row.values())[0]
        if row_id is not None:
            updated_ids.append(str(row_id))
    return updated_ids


async def bulk_update_entity_scopes(
    pool: Pool, enums: EnumRegistry, entity_ids: list[str], scopes: list[str], op: str
) -> list[str]:
    """Bulk update entity privacy scopes.

    Args:
        pool: Database connection pool.
        enums: Enum registry for validation.
        entity_ids: Entity UUIDs.
        scopes: Scope names.
        op: add, remove, or set.

    Returns:
        Updated entity ids.
    """

    scope_ids = require_scopes(scopes, enums)
    rows = await pool.fetch(
        QUERIES["entities/bulk_update_scopes"], entity_ids, op, scope_ids
    )
    updated_ids: list[str] = []
    for row in rows:
        row_id = row.get("id")
        if row_id is None and len(row.values()) > 0:
            row_id = list(row.values())[0]
        if row_id is not None:
            updated_ids.append(str(row_id))
    return updated_ids


async def query_audit_log(
    pool: Pool,
    table_name: str | None = None,
    action: str | None = None,
    actor_type: str | None = None,
    actor_id: str | None = None,
    record_id: str | None = None,
    scope_id: str | None = None,
    limit: int = 50,
    offset: int = 0,
) -> list[dict]:
    """List audit log entries with optional filters.

    Args:
        pool: Database connection pool.
        table_name: Table name filter.
        action: Action filter (insert, update, delete).
        actor_type: Actor type filter (agent, entity, system).
        actor_id: Actor UUID filter.
        record_id: Record id filter.
        limit: Max rows to return.
        offset: Pagination offset.

    Returns:
        List of audit entries as dicts.
    """

    actor_id = actor_id or None
    rows = await pool.fetch(
        QUERIES["audit/list"],
        table_name,
        action,
        actor_type,
        actor_id,
        record_id,
        scope_id,
        limit,
        offset,
    )
    return [dict(r) for r in rows]


async def list_audit_scopes(pool: Pool) -> list[dict]:
    """List privacy scopes with usage counts."""

    rows = await pool.fetch(QUERIES["audit/scopes"])
    return [dict(r) for r in rows]


async def list_audit_actors(pool: Pool, actor_type: str | None = None) -> list[dict]:
    """List audit actors with activity counts."""

    rows = await pool.fetch(QUERIES["audit/actors"], actor_type)
    normalized: list[dict] = []
    for row in rows:
        item = dict(row)
        changed_by_type = str(item.get("changed_by_type") or "").strip().lower()
        if changed_by_type in {"", "unknown", "none", "null"}:
            item["changed_by_type"] = "system"
            item["changed_by_id"] = ""
        normalized.append(item)
    return normalized


def _normalize_diff_value(value: object) -> object:
    """Normalize diff values for stable comparisons.

    Args:
        value: Value to normalize, often dict or list.

    Returns:
        JSON-encoded string for complex types, otherwise original value.
    """
    if isinstance(value, (dict, list)):
        return json.dumps(value, sort_keys=True)
    return value


async def get_approval_diff(pool: Pool, approval_id: str) -> dict:
    """Compute diff between approval request and current entity state.

    Args:
        pool: Database connection pool.
        approval_id: Approval request UUID.

    Returns:
        Dict with request_type and changes map.
    """

    row = await pool.fetchrow(QUERIES["approvals/get_request"], approval_id)
    if not row:
        raise ValueError("Approval request not found")

    approval = dict(row)
    change_details = approval.get("change_details") or {}
    if isinstance(change_details, str):
        change_details = json.loads(change_details)

    request_type = approval.get("request_type")
    changes: dict[str, dict[str, object]] = {}

    if request_type == "update_entity":
        entity_id = change_details.get("entity_id")
        if not entity_id:
            raise ValueError("Approval request missing entity_id")

        entity_row = await pool.fetchrow(QUERIES["entities/get"], entity_id)
        if not entity_row:
            raise ValueError("Entity not found for approval diff")

        entity = dict(entity_row)
        for key, new_val in change_details.items():
            if key == "entity_id":
                continue
            old_val = entity.get(key)
            if _normalize_diff_value(old_val) != _normalize_diff_value(new_val):
                changes[key] = {"from": old_val, "to": new_val}
    elif request_type == "create_entity":
        for key, new_val in change_details.items():
            changes[key] = {"from": None, "to": new_val}
    elif request_type in {"create_context", "create_relationship", "create_job"}:
        for key, new_val in change_details.items():
            changes[key] = {"from": None, "to": new_val}
    elif request_type == "update_relationship":
        relationship_id = change_details.get("relationship_id")
        if not relationship_id:
            raise ValueError("Approval request missing relationship_id")

        rel_row = await pool.fetchrow(
            QUERIES["relationships/get_by_id"], relationship_id
        )
        if not rel_row:
            raise ValueError("Relationship not found for approval diff")

        relationship = dict(rel_row)
        for key, new_val in change_details.items():
            if key == "relationship_id":
                continue
            old_val = relationship.get(key)
            if _normalize_diff_value(old_val) != _normalize_diff_value(new_val):
                changes[key] = {"from": old_val, "to": new_val}
    elif request_type == "update_job_status":
        job_id = change_details.get("job_id")
        if not job_id:
            raise ValueError("Approval request missing job_id")

        job_row = await pool.fetchrow(QUERIES["jobs/get"], job_id)
        if not job_row:
            raise ValueError("Job not found for approval diff")

        job = dict(job_row)
        for key, new_val in change_details.items():
            if key == "job_id":
                continue
            old_val = job.get(key)
            if _normalize_diff_value(old_val) != _normalize_diff_value(new_val):
                changes[key] = {"from": old_val, "to": new_val}

    return {
        "approval_id": approval_id,
        "request_type": request_type,
        "changes": changes,
    }
