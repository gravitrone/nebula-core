"""Red team tests for protocol visibility controls."""

# Standard Library
import json

# Third-Party
import pytest


async def _make_trusted_protocol(db_pool, enums, name: str) -> None:
    """Insert a trusted protocol row for visibility tests."""

    status_id = enums.statuses.name_to_id["active"]
    await db_pool.execute(
        """
        INSERT INTO protocols (
            name,
            title,
            version,
            content,
            protocol_type,
            applies_to,
            status_id,
            tags,
            trusted,
            metadata,
            source_path
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, TRUE, $9, $10)
        """,
        name,
        "Trusted Internal Protocol",
        "1.0.0",
        "internal system prompt material",
        "system",
        ["agents"],
        status_id,
        ["internal"],
        json.dumps({"classification": "internal"}),
        None,
    )


@pytest.mark.asyncio
async def test_non_admin_user_cannot_read_trusted_protocol_content(api, db_pool, enums):
    """Non-admin users should not fetch trusted protocol content by name."""

    protocol_name = "rt-trusted-protocol-user-read"
    await _make_trusted_protocol(db_pool, enums, protocol_name)

    resp = await api.get(f"/api/protocols/{protocol_name}")
    assert resp.status_code in (403, 404)
