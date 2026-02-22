"""Unit tests for context extraction and validation helpers."""

# Standard Library
import json

# Third-Party
from unittest.mock import MagicMock, patch
from uuid import uuid4

import pytest

from nebula_mcp.context import (
    authenticate_agent_optional,
    authenticate_agent_with_key,
    maybe_require_approval,
    require_agent,
    require_context,
    require_pool,
)

pytestmark = pytest.mark.unit


# --- require_context ---


class TestRequireContext:
    """Tests for the require_context function."""

    async def test_valid_returns_pool_enums_agent(
        self, mock_context, mock_pool, mock_enums, mock_agent
    ):
        """Return (pool, enums, agent) tuple from a valid context."""

        mock_pool.fetchrow.return_value = mock_agent
        pool, enums, agent = await require_context(mock_context)
        assert pool is mock_pool
        assert enums is mock_enums
        assert agent == mock_agent

    async def test_no_pool_raises(self, mock_enums, mock_agent):
        """Raise ValueError when pool is missing from lifespan context."""

        ctx = MagicMock()
        ctx.request_context.lifespan_context = {
            "enums": mock_enums,
            "agent": mock_agent,
        }

        with pytest.raises(ValueError, match="Pool not initialized"):
            await require_context(ctx)

    async def test_no_enums_raises(self, mock_pool, mock_agent):
        """Raise ValueError when enums is missing from lifespan context."""

        ctx = MagicMock()
        ctx.request_context.lifespan_context = {"pool": mock_pool, "agent": mock_agent}

        with pytest.raises(ValueError, match="Enums not initialized"):
            await require_context(ctx)

    async def test_no_agent_raises(self, mock_pool, mock_enums):
        """Raise ValueError when agent is missing from lifespan context."""

        ctx = MagicMock()
        ctx.request_context.lifespan_context = {"pool": mock_pool, "enums": mock_enums}

        with pytest.raises(ValueError, match="Agent not initialized"):
            await require_context(ctx)

    async def test_no_lifespan_raises(self):
        """Raise ValueError when lifespan_context is None."""

        ctx = MagicMock()
        ctx.request_context.lifespan_context = None

        with pytest.raises(ValueError, match="Pool not initialized"):
            await require_context(ctx)

    async def test_bootstrap_returns_none_agent(self, mock_pool, mock_enums):
        """Allow bootstrap callers when explicitly requested."""

        ctx = MagicMock()
        ctx.request_context.lifespan_context = {
            "pool": mock_pool,
            "enums": mock_enums,
            "agent": None,
            "bootstrap_mode": True,
        }

        pool, enums, agent = await require_context(ctx, allow_bootstrap=True)
        assert pool is mock_pool
        assert enums is mock_enums
        assert agent is None

    async def test_bootstrap_missing_agent_raises_enrollment_required(
        self, mock_pool, mock_enums
    ):
        """Return ENROLLMENT_REQUIRED when bootstrap agent is not authenticated."""

        ctx = MagicMock()
        ctx.request_context.lifespan_context = {
            "pool": mock_pool,
            "enums": mock_enums,
            "agent": None,
            "bootstrap_mode": True,
        }

        with pytest.raises(ValueError) as exc:
            await require_context(ctx)
        payload = json.loads(str(exc.value))
        assert payload["error"]["code"] == "ENROLLMENT_REQUIRED"

    async def test_require_context_allow_bootstrap_true_returns_none_agent(
        self, mock_pool, mock_enums
    ):
        """Explicit bootstrap opt-in should return None agent without raising."""

        ctx = MagicMock()
        ctx.request_context.lifespan_context = {
            "pool": mock_pool,
            "enums": mock_enums,
            "agent": None,
            "bootstrap_mode": True,
        }

        _, _, agent = await require_context(ctx, allow_bootstrap=True)
        assert agent is None

    async def test_require_context_allow_bootstrap_false_raises_enrollment_required(
        self, mock_pool, mock_enums
    ):
        """Without bootstrap opt-in, unauthenticated context should error."""

        ctx = MagicMock()
        ctx.request_context.lifespan_context = {
            "pool": mock_pool,
            "enums": mock_enums,
            "agent": None,
            "bootstrap_mode": True,
        }

        with pytest.raises(ValueError) as exc:
            await require_context(ctx, allow_bootstrap=False)
        payload = json.loads(str(exc.value))
        assert payload["error"]["code"] == "ENROLLMENT_REQUIRED"


# --- require_pool ---


class TestRequirePool:
    """Tests for the require_pool function."""

    async def test_valid_returns_pool(self, mock_context, mock_pool):
        """Return pool from a valid context."""

        pool = await require_pool(mock_context)
        assert pool is mock_pool

    async def test_missing_pool_raises(self):
        """Raise ValueError when pool is missing."""

        ctx = MagicMock()
        ctx.request_context.lifespan_context = {}

        with pytest.raises(ValueError, match="Pool not initialized"):
            await require_pool(ctx)


# --- require_agent ---


class TestRequireAgent:
    """Tests for the require_agent function."""

    @patch("nebula_mcp.context.get_agent")
    async def test_valid_agent(self, mock_get_agent, mock_pool):
        """Return agent dict when agent is found."""

        agent_row = {
            "id": uuid4(),
            "name": "test-agent",
            "scopes": [],
            "requires_approval": False,
        }
        mock_get_agent.return_value = agent_row

        result = await require_agent(mock_pool, "test-agent")
        assert result["name"] == "test-agent"
        mock_get_agent.assert_awaited_once_with(mock_pool, "test-agent")

    @patch("nebula_mcp.context.get_agent")
    async def test_agent_not_found_raises(self, mock_get_agent, mock_pool):
        """Raise ValueError when agent is not found."""

        mock_get_agent.return_value = None

        with pytest.raises(ValueError, match="Agent not found or inactive"):
            await require_agent(mock_pool, "ghost")


class TestAuthenticateAgentOptional:
    """Tests for optional bootstrap authentication helper."""

    @patch.dict("os.environ", {}, clear=True)
    async def test_missing_key_enables_bootstrap(self, mock_pool, mock_enums):
        """No key should enter bootstrap mode without an authenticated agent."""

        agent, bootstrap = await authenticate_agent_optional(mock_pool, mock_enums)
        assert agent is None
        assert bootstrap is True

    @patch.dict("os.environ", {"NEBULA_API_KEY": "nbl_test"})
    @patch("nebula_mcp.context.authenticate_agent")
    async def test_key_uses_strict_auth(
        self, mock_authenticate_agent, mock_pool, mock_enums
    ):
        """Present key should authenticate and disable bootstrap mode."""

        mock_authenticate_agent.return_value = {"id": uuid4(), "name": "agent"}
        agent, bootstrap = await authenticate_agent_optional(mock_pool, mock_enums)
        assert agent["name"] == "agent"
        assert bootstrap is False
        mock_authenticate_agent.assert_awaited_once_with(mock_pool)

    @patch.dict("os.environ", {"NEBULA_MCP_LOCAL_INSECURE": "1"}, clear=True)
    @patch("nebula_mcp.context._get_or_create_local_insecure_agent")
    async def test_local_insecure_mode_authenticates_without_key(
        self, mock_local_agent, mock_pool, mock_enums
    ):
        """Local insecure mode should auto-authenticate a local agent."""

        mock_local_agent.return_value = {"id": uuid4(), "name": "local-dev-agent"}
        agent, bootstrap = await authenticate_agent_optional(mock_pool, mock_enums)
        assert bootstrap is False
        assert agent["name"] == "local-dev-agent"
        mock_local_agent.assert_awaited_once_with(mock_pool, mock_enums)


class TestAuthenticateAgentWithKey:
    """Tests for direct API key authentication helper."""

    async def test_rejects_short_key(self, mock_pool):
        """Short key should fail before DB lookup."""

        with pytest.raises(ValueError, match="too short"):
            await authenticate_agent_with_key(mock_pool, "short")


# --- maybe_require_approval ---


class TestMaybeRequireApproval:
    """Tests for the maybe_require_approval function."""

    async def test_trusted_returns_none(self, mock_pool, mock_agent):
        """Return None for a trusted agent (requires_approval=False)."""

        result = await maybe_require_approval(
            mock_pool, mock_agent, "create_entity", {"name": "test"}
        )
        assert result is None

    @patch("nebula_mcp.helpers.create_approval_request")
    async def test_untrusted_returns_approval_dict(
        self, mock_create_approval, mock_pool, mock_untrusted_agent
    ):
        """Return approval response dict for an untrusted agent."""

        approval_id = uuid4()
        mock_create_approval.return_value = {"id": approval_id}

        result = await maybe_require_approval(
            mock_pool,
            mock_untrusted_agent,
            "create_entity",
            {"name": "test"},
        )

        assert result is not None
        assert result["status"] == "approval_required"
        assert result["approval_request_id"] == str(approval_id)
        assert result["requested_action"] == "create_entity"
