"""Integration tests for MCP-native enrollment bootstrap flow."""

# Standard Library
import json
from datetime import UTC, datetime, timedelta
from uuid import uuid4

# Third-Party
import pytest

# Local
from nebula_mcp.helpers import approve_request as do_approve
from nebula_mcp.helpers import reject_request as do_reject
from nebula_mcp.models import (
    AgentAuthAttachInput,
    AgentEnrollRedeemInput,
    AgentEnrollStartInput,
    AgentEnrollWaitInput,
    QueryEntitiesInput,
)
from nebula_mcp.server import (
    agent_auth_attach,
    agent_enroll_redeem,
    agent_enroll_start,
    agent_enroll_wait,
    query_entities,
)

pytestmark = pytest.mark.integration


async def _get_enrollment_row(db_pool, registration_id: str) -> dict:
    """Handle get enrollment row.

    Args:
        db_pool: Input parameter for _get_enrollment_row.
        registration_id: Input parameter for _get_enrollment_row.

    Returns:
        Result value from the operation.
    """

    row = await db_pool.fetchrow(
        "SELECT * FROM agent_enrollment_sessions WHERE id = $1::uuid",
        registration_id,
    )
    return dict(row) if row else {}


async def test_bootstrap_blocks_non_enroll_tools(bootstrap_mcp_context):
    """Non-enrollment tools should fail with ENROLLMENT_REQUIRED in bootstrap mode."""

    with pytest.raises(ValueError) as exc:
        await query_entities(QueryEntitiesInput(), bootstrap_mcp_context)

    payload = json.loads(str(exc.value))
    assert payload["error"]["code"] == "ENROLLMENT_REQUIRED"
    assert payload["error"]["next_steps"] == [
        "agent_enroll_start",
        "agent_enroll_wait",
        "agent_enroll_redeem",
        "agent_auth_attach",
    ]


async def test_enroll_tools_reject_authenticated_agent_context(mock_mcp_context):
    """Enroll bootstrap tools should reject authenticated agent contexts."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    with pytest.raises(ValueError, match="Agent already authenticated"):
        await agent_enroll_start(
            AgentEnrollStartInput(name=name, requested_scopes=["public"]),
            mock_mcp_context,
        )

    with pytest.raises(ValueError, match="Agent already authenticated"):
        await agent_enroll_wait(
            AgentEnrollWaitInput(
                registration_id=str(uuid4()),
                enrollment_token="nbe_fake",
                timeout_seconds=1,
            ),
            mock_mcp_context,
        )

    with pytest.raises(ValueError, match="Agent already authenticated"):
        await agent_enroll_redeem(
            AgentEnrollRedeemInput(
                registration_id=str(uuid4()),
                enrollment_token="nbe_fake",
            ),
            mock_mcp_context,
        )


async def test_enroll_start_creates_pending_approval(bootstrap_mcp_context, db_pool):
    """Enrollment start should create approval + registration token."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    started = await agent_enroll_start(
        AgentEnrollStartInput(
            name=name,
            requested_scopes=["public"],
            requested_requires_approval=True,
        ),
        bootstrap_mcp_context,
    )
    assert started["status"] == "pending_approval"
    assert started["registration_id"]
    assert started["enrollment_token"].startswith("nbe_")

    session = await _get_enrollment_row(db_pool, started["registration_id"])
    assert session["status"] == "pending_approval"

    approval = await db_pool.fetchrow(
        "SELECT request_type FROM approval_requests WHERE id = $1::uuid",
        session["approval_request_id"],
    )
    assert approval["request_type"] == "register_agent"


async def test_enroll_start_rejects_invalid_scope_name(bootstrap_mcp_context):
    """Enrollment start should reject unknown scope names."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    with pytest.raises(ValueError, match="Unknown scope"):
        await agent_enroll_start(
            AgentEnrollStartInput(
                name=name,
                requested_scopes=["definitely-not-a-scope"],
            ),
            bootstrap_mcp_context,
        )


async def test_enroll_start_rejects_duplicate_agent_name(bootstrap_mcp_context):
    """Enrollment start should reject duplicate agent name reuse."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    await agent_enroll_start(
        AgentEnrollStartInput(name=name, requested_scopes=["public"]),
        bootstrap_mcp_context,
    )

    with pytest.raises(ValueError, match="already exists"):
        await agent_enroll_start(
            AgentEnrollStartInput(name=name, requested_scopes=["public"]),
            bootstrap_mcp_context,
        )


async def test_enroll_wait_timeout_returns_pending(bootstrap_mcp_context):
    """Wait should return pending state and retry hint when no reviewer action occurred."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    started = await agent_enroll_start(
        AgentEnrollStartInput(name=name, requested_scopes=["public"]),
        bootstrap_mcp_context,
    )
    waited = await agent_enroll_wait(
        AgentEnrollWaitInput(
            registration_id=started["registration_id"],
            enrollment_token=started["enrollment_token"],
            timeout_seconds=1,
        ),
        bootstrap_mcp_context,
    )
    assert waited["status"] == "pending_approval"
    assert waited["retry_after_ms"] >= 1000
    assert waited["can_redeem"] is False


async def test_enroll_wait_caps_timeout_bounds(bootstrap_mcp_context):
    """Enrollment wait should accept boundary timeout values 1 and 60."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    started = await agent_enroll_start(
        AgentEnrollStartInput(name=name, requested_scopes=["public"]),
        bootstrap_mcp_context,
    )

    waited_min = await agent_enroll_wait(
        AgentEnrollWaitInput(
            registration_id=started["registration_id"],
            enrollment_token=started["enrollment_token"],
            timeout_seconds=1,
        ),
        bootstrap_mcp_context,
    )
    assert waited_min["status"] == "pending_approval"

    waited_max = await agent_enroll_wait(
        AgentEnrollWaitInput(
            registration_id=started["registration_id"],
            enrollment_token=started["enrollment_token"],
            timeout_seconds=60,
        ),
        bootstrap_mcp_context,
    )
    assert waited_max["status"] == "pending_approval"


async def test_enroll_wait_rejects_invalid_token(bootstrap_mcp_context):
    """Enrollment wait should reject an invalid token."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    started = await agent_enroll_start(
        AgentEnrollStartInput(name=name, requested_scopes=["public"]),
        bootstrap_mcp_context,
    )

    with pytest.raises(ValueError, match="Invalid enrollment token"):
        await agent_enroll_wait(
            AgentEnrollWaitInput(
                registration_id=started["registration_id"],
                enrollment_token="nbe_totally_wrong",
                timeout_seconds=1,
            ),
            bootstrap_mcp_context,
        )


async def test_enroll_wait_rejects_cross_session_token(bootstrap_mcp_context):
    """Enrollment wait should reject token/session mismatches."""

    first = await agent_enroll_start(
        AgentEnrollStartInput(
            name=f"mcp-enroll-{uuid4().hex[:8]}",
            requested_scopes=["public"],
        ),
        bootstrap_mcp_context,
    )
    second = await agent_enroll_start(
        AgentEnrollStartInput(
            name=f"mcp-enroll-{uuid4().hex[:8]}",
            requested_scopes=["public"],
        ),
        bootstrap_mcp_context,
    )

    with pytest.raises(ValueError, match="Invalid enrollment token"):
        await agent_enroll_wait(
            AgentEnrollWaitInput(
                registration_id=second["registration_id"],
                enrollment_token=first["enrollment_token"],
                timeout_seconds=1,
            ),
            bootstrap_mcp_context,
        )


async def test_agent_auth_attach_rejects_invalid_key(bootstrap_mcp_context):
    """Auth attach should reject invalid API keys."""

    with pytest.raises(ValueError, match="invalid or revoked"):
        await agent_auth_attach(
            AgentAuthAttachInput(api_key="nbl_invalid_key"),
            bootstrap_mcp_context,
        )


async def test_enroll_redeem_attach_unblocks_non_enroll_tools(
    bootstrap_mcp_context, db_pool, enums, test_entity
):
    """Redeemed key should authenticate same MCP session via attach tool."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    started = await agent_enroll_start(
        AgentEnrollStartInput(
            name=name,
            requested_scopes=["public"],
            requested_requires_approval=True,
        ),
        bootstrap_mcp_context,
    )
    session = await _get_enrollment_row(db_pool, started["registration_id"])

    await do_approve(
        db_pool,
        enums,
        str(session["approval_request_id"]),
        str(test_entity["id"]),
        review_details={"grant_requires_approval": False},
    )

    redeemed = await agent_enroll_redeem(
        AgentEnrollRedeemInput(
            registration_id=started["registration_id"],
            enrollment_token=started["enrollment_token"],
        ),
        bootstrap_mcp_context,
    )
    attached = await agent_auth_attach(
        AgentAuthAttachInput(api_key=redeemed["api_key"]),
        bootstrap_mcp_context,
    )

    assert attached["status"] == "authenticated"
    assert attached["agent_name"] == name

    result = await query_entities(QueryEntitiesInput(limit=1), bootstrap_mcp_context)
    assert isinstance(result, list)


async def test_enroll_approve_with_grants_applies_final_scope_and_trust(
    bootstrap_mcp_context, db_pool, enums, test_entity
):
    """Reviewer grants should override requested values at approval execution."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    started = await agent_enroll_start(
        AgentEnrollStartInput(
            name=name,
            requested_scopes=["public"],
            requested_requires_approval=True,
        ),
        bootstrap_mcp_context,
    )
    session = await _get_enrollment_row(db_pool, started["registration_id"])

    await do_approve(
        db_pool,
        enums,
        str(session["approval_request_id"]),
        str(test_entity["id"]),
        review_details={
            "grant_scopes": ["public", "private"],
            "grant_scope_ids": [
                str(enums.scopes.name_to_id["public"]),
                str(enums.scopes.name_to_id["private"]),
            ],
            "grant_requires_approval": False,
        },
        review_notes="approved with grants",
    )

    waited = await agent_enroll_wait(
        AgentEnrollWaitInput(
            registration_id=started["registration_id"],
            enrollment_token=started["enrollment_token"],
            timeout_seconds=1,
        ),
        bootstrap_mcp_context,
    )
    assert waited["status"] == "approved"
    assert waited["can_redeem"] is True

    refreshed_agent = await db_pool.fetchrow(
        "SELECT scopes, requires_approval FROM agents WHERE id = $1::uuid",
        session["agent_id"],
    )
    assert refreshed_agent["requires_approval"] is False
    assert set(refreshed_agent["scopes"]) == {
        enums.scopes.name_to_id["public"],
        enums.scopes.name_to_id["private"],
    }


async def test_enroll_reject_returns_reason(
    bootstrap_mcp_context, db_pool, test_entity
):
    """Rejected enrollment should return reviewer reason via wait."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    started = await agent_enroll_start(
        AgentEnrollStartInput(name=name, requested_scopes=["public"]),
        bootstrap_mcp_context,
    )
    session = await _get_enrollment_row(db_pool, started["registration_id"])

    await do_reject(
        db_pool,
        str(session["approval_request_id"]),
        str(test_entity["id"]),
        "missing trust signals",
    )

    waited = await agent_enroll_wait(
        AgentEnrollWaitInput(
            registration_id=started["registration_id"],
            enrollment_token=started["enrollment_token"],
            timeout_seconds=1,
        ),
        bootstrap_mcp_context,
    )
    assert waited["status"] == "rejected"
    assert waited["reason"] == "missing trust signals"
    assert waited["can_redeem"] is False


async def test_enroll_redeem_rejects_invalid_token(
    bootstrap_mcp_context, db_pool, enums, test_entity
):
    """Enrollment redeem should reject invalid token values."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    started = await agent_enroll_start(
        AgentEnrollStartInput(name=name, requested_scopes=["public"]),
        bootstrap_mcp_context,
    )
    session = await _get_enrollment_row(db_pool, started["registration_id"])
    await do_approve(
        db_pool,
        enums,
        str(session["approval_request_id"]),
        str(test_entity["id"]),
    )

    with pytest.raises(ValueError, match="Invalid enrollment token"):
        await agent_enroll_redeem(
            AgentEnrollRedeemInput(
                registration_id=started["registration_id"],
                enrollment_token="nbe_invalid",
            ),
            bootstrap_mcp_context,
        )


async def test_enroll_redeem_rejects_cross_session_token(
    bootstrap_mcp_context, db_pool, enums, test_entity
):
    """Enrollment redeem should reject token/session mismatches."""

    first = await agent_enroll_start(
        AgentEnrollStartInput(
            name=f"mcp-enroll-{uuid4().hex[:8]}",
            requested_scopes=["public"],
        ),
        bootstrap_mcp_context,
    )
    second = await agent_enroll_start(
        AgentEnrollStartInput(
            name=f"mcp-enroll-{uuid4().hex[:8]}",
            requested_scopes=["public"],
        ),
        bootstrap_mcp_context,
    )

    second_session = await _get_enrollment_row(db_pool, second["registration_id"])
    await do_approve(
        db_pool,
        enums,
        str(second_session["approval_request_id"]),
        str(test_entity["id"]),
    )

    with pytest.raises(ValueError, match="Invalid enrollment token"):
        await agent_enroll_redeem(
            AgentEnrollRedeemInput(
                registration_id=second["registration_id"],
                enrollment_token=first["enrollment_token"],
            ),
            bootstrap_mcp_context,
        )


async def test_enroll_redeem_is_one_time(
    bootstrap_mcp_context, db_pool, enums, test_entity
):
    """Redeem should mint one API key and block replay."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    started = await agent_enroll_start(
        AgentEnrollStartInput(name=name, requested_scopes=["public"]),
        bootstrap_mcp_context,
    )
    session = await _get_enrollment_row(db_pool, started["registration_id"])
    await do_approve(
        db_pool,
        enums,
        str(session["approval_request_id"]),
        str(test_entity["id"]),
    )

    redeemed = await agent_enroll_redeem(
        AgentEnrollRedeemInput(
            registration_id=started["registration_id"],
            enrollment_token=started["enrollment_token"],
        ),
        bootstrap_mcp_context,
    )
    assert redeemed["api_key"].startswith("nbl_")
    assert redeemed["agent_id"] == str(session["agent_id"])
    assert "public" in redeemed["scopes"]

    with pytest.raises(ValueError, match="already redeemed"):
        await agent_enroll_redeem(
            AgentEnrollRedeemInput(
                registration_id=started["registration_id"],
                enrollment_token=started["enrollment_token"],
            ),
            bootstrap_mcp_context,
        )


async def test_enroll_wait_reports_redeemed_after_successful_redeem(
    bootstrap_mcp_context, db_pool, enums, test_entity
):
    """Enrollment wait should report redeemed once key is issued."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    started = await agent_enroll_start(
        AgentEnrollStartInput(name=name, requested_scopes=["public"]),
        bootstrap_mcp_context,
    )
    session = await _get_enrollment_row(db_pool, started["registration_id"])
    await do_approve(
        db_pool,
        enums,
        str(session["approval_request_id"]),
        str(test_entity["id"]),
    )

    await agent_enroll_redeem(
        AgentEnrollRedeemInput(
            registration_id=started["registration_id"],
            enrollment_token=started["enrollment_token"],
        ),
        bootstrap_mcp_context,
    )

    waited = await agent_enroll_wait(
        AgentEnrollWaitInput(
            registration_id=started["registration_id"],
            enrollment_token=started["enrollment_token"],
            timeout_seconds=1,
        ),
        bootstrap_mcp_context,
    )
    assert waited["status"] == "redeemed"
    assert waited["can_redeem"] is False


async def test_enroll_expired_wait_and_redeem(bootstrap_mcp_context, db_pool):
    """Expired enrollment should report expired and deny redemption."""

    name = f"mcp-enroll-{uuid4().hex[:8]}"
    started = await agent_enroll_start(
        AgentEnrollStartInput(name=name, requested_scopes=["public"]),
        bootstrap_mcp_context,
    )
    await db_pool.execute(
        """
        UPDATE agent_enrollment_sessions
        SET expires_at = $2
        WHERE id = $1::uuid
        """,
        started["registration_id"],
        datetime.now(UTC) - timedelta(minutes=1),
    )

    waited = await agent_enroll_wait(
        AgentEnrollWaitInput(
            registration_id=started["registration_id"],
            enrollment_token=started["enrollment_token"],
            timeout_seconds=1,
        ),
        bootstrap_mcp_context,
    )
    assert waited["status"] == "expired"
    assert waited["can_redeem"] is False

    with pytest.raises(ValueError, match="expired"):
        await agent_enroll_redeem(
            AgentEnrollRedeemInput(
                registration_id=started["registration_id"],
                enrollment_token=started["enrollment_token"],
            ),
            bootstrap_mcp_context,
        )
