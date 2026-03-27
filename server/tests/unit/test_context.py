"""Unit tests for context extraction and validation helpers."""

# Standard Library
import json

# Third-Party
from unittest.mock import MagicMock, patch
from uuid import uuid4

import pytest

from nebula_mcp.context import (
    _env_truthy,
    _get_or_create_local_insecure_agent,
    _local_insecure_agent_name,
    authenticate_agent,
    authenticate_agent_optional,
    authenticate_agent_with_key,
    enrollment_required_error,
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

    async def test_valid_refreshes_agent_in_lifespan_context(
        self, mock_context, mock_pool, mock_enums
    ):
        """Valid context should refresh agent row and write it back."""

        refreshed = {"id": uuid4(), "name": "fresh-agent", "requires_approval": False}
        mock_context.request_context.lifespan_context["agent"] = {"id": refreshed["id"]}
        mock_pool.fetchrow.return_value = refreshed

        _, _, agent = await require_context(mock_context)

        assert agent == refreshed
        assert mock_context.request_context.lifespan_context["agent"]["name"] == "fresh-agent"
        assert mock_pool.fetchrow.await_args.args[1] == str(refreshed["id"])

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

    async def test_bootstrap_missing_agent_raises_enrollment_required(self, mock_pool, mock_enums):
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

    async def test_allow_bootstrap_true_without_bootstrap_mode_returns_none_agent(
        self, mock_pool, mock_enums
    ):
        """Explicit bootstrap opt-in should allow unauthenticated contexts."""

        ctx = MagicMock()
        ctx.request_context.lifespan_context = {
            "pool": mock_pool,
            "enums": mock_enums,
            "agent": None,
            "bootstrap_mode": False,
        }

        _, _, agent = await require_context(ctx, allow_bootstrap=True)
        assert agent is None

    async def test_agent_refresh_missing_row_raises(self, mock_pool, mock_enums):
        """A stale lifespan agent id should fail with not-found error."""

        ctx = MagicMock()
        ctx.request_context.lifespan_context = {
            "pool": mock_pool,
            "enums": mock_enums,
            "agent": {"id": uuid4()},
        }
        mock_pool.fetchrow.return_value = None

        with pytest.raises(ValueError, match="Agent not found or inactive"):
            await require_context(ctx)


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
    async def test_key_uses_strict_auth(self, mock_authenticate_agent, mock_pool, mock_enums):
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

    async def test_rejects_short_key_uses_custom_key_name(self, mock_pool):
        """Short-key validation should include provided key label."""

        with pytest.raises(ValueError, match="Custom key is too short"):
            await authenticate_agent_with_key(mock_pool, "short", key_name="Custom key")

    async def test_rejects_unknown_prefix(self, mock_pool):
        """Unknown key prefix should return invalid or revoked."""

        mock_pool.fetchrow.return_value = None
        with pytest.raises(ValueError, match="invalid or revoked"):
            await authenticate_agent_with_key(mock_pool, "nbl_abcdef123456")

    @patch("argon2.PasswordHasher.verify", side_effect=Exception("boom"))
    async def test_hash_verification_error_bubbles(self, _verify, mock_pool):
        """Unexpected hasher errors should bubble for visibility."""

        mock_pool.fetchrow.return_value = {
            "key_hash": "hash",
            "agent_id": uuid4(),
        }
        with pytest.raises(Exception, match="boom"):
            await authenticate_agent_with_key(mock_pool, "nbl_abcdef123456")

    @patch(
        "argon2.PasswordHasher.verify",
        side_effect=__import__("argon2").exceptions.VerifyMismatchError,
    )
    async def test_rejects_hash_mismatch(self, _verify, mock_pool):
        """Argon2 mismatches should return a clear hash-mismatch error."""

        mock_pool.fetchrow.return_value = {
            "key_hash": "hash",
            "agent_id": uuid4(),
        }
        with pytest.raises(ValueError, match="hash mismatch"):
            await authenticate_agent_with_key(mock_pool, "nbl_abcdef123456")

    @patch("argon2.PasswordHasher.verify")
    async def test_rejects_non_agent_key(self, _verify, mock_pool):
        """Keys bound to users only should not authenticate MCP agents."""

        mock_pool.fetchrow.return_value = {
            "key_hash": "hash",
            "agent_id": None,
        }
        with pytest.raises(ValueError, match="not an agent key"):
            await authenticate_agent_with_key(mock_pool, "nbl_abcdef123456")

    @patch("argon2.PasswordHasher.verify")
    async def test_rejects_missing_agent_after_key_match(self, _verify, mock_pool):
        """Revoked/inactive agents should fail after key hash verification."""

        mock_pool.fetchrow.side_effect = [
            {"key_hash": "hash", "agent_id": str(uuid4())},
            None,
        ]
        with pytest.raises(ValueError, match="Agent not found or inactive"):
            await authenticate_agent_with_key(mock_pool, "nbl_abcdef123456")

    @patch("argon2.PasswordHasher.verify")
    async def test_success_returns_agent_dict(self, _verify, mock_pool):
        """Valid key and active agent should return agent dict."""

        agent_id = str(uuid4())
        mock_pool.fetchrow.side_effect = [
            {"key_hash": "hash", "agent_id": agent_id},
            {"id": agent_id, "name": "agent-1", "requires_approval": False},
        ]
        agent = await authenticate_agent_with_key(mock_pool, "nbl_abcdef123456")
        assert agent["name"] == "agent-1"

    @patch("argon2.PasswordHasher.verify")
    async def test_exact_min_length_key_uses_full_prefix(self, _verify, mock_pool):
        """Keys with length 8 should still run lookup using 8-char prefix."""

        key = "nbl_1234"
        agent_id = str(uuid4())
        mock_pool.fetchrow.side_effect = [
            {"key_hash": "hash", "agent_id": agent_id},
            {"id": agent_id, "name": "agent-8", "requires_approval": False},
        ]

        agent = await authenticate_agent_with_key(mock_pool, key)

        assert agent["name"] == "agent-8"
        assert mock_pool.fetchrow.await_args_list[0].args[1] == key


class TestAuthenticateAgent:
    """Tests for environment-based agent authentication wrapper."""

    @patch.dict("os.environ", {}, clear=True)
    async def test_missing_env_key_raises(self, mock_pool):
        """Missing NEBULA_API_KEY should raise helpful setup error."""

        with pytest.raises(ValueError, match="NEBULA_API_KEY"):
            await authenticate_agent(mock_pool)

    @patch.dict("os.environ", {"NEBULA_API_KEY": "nbl_env_key"}, clear=True)
    @patch("nebula_mcp.context.authenticate_agent_with_key")
    async def test_passes_env_key_to_authenticator(self, mock_auth, mock_pool):
        """Wrapper should delegate to authenticate_agent_with_key."""

        mock_auth.return_value = {"id": uuid4(), "name": "agent-env"}
        agent = await authenticate_agent(mock_pool)
        assert agent["name"] == "agent-env"
        mock_auth.assert_awaited_once_with(mock_pool, "nbl_env_key", key_name="NEBULA_API_KEY")


class TestEnrollmentErrorPayload:
    """Tests for bootstrap enrollment required error helper."""

    def test_enrollment_required_error_contains_structured_payload(self):
        """Error payload should include code, message, and next steps."""

        err = enrollment_required_error()
        payload = json.loads(str(err))
        assert payload["error"]["code"] == "ENROLLMENT_REQUIRED"
        assert payload["error"]["message"] == "Agent not enrolled"
        assert payload["error"]["next_steps"] == [
            "agent_enroll_start",
            "agent_enroll_wait",
            "agent_enroll_redeem",
            "agent_auth_attach",
        ]


class TestLocalInsecureHelpers:
    """Tests for local insecure mode utility helpers."""

    @patch.dict("os.environ", {"NEBULA_MCP_LOCAL_INSECURE": "TRUE"}, clear=True)
    def test_env_truthy_true_variants(self):
        """Truthy environment values should be recognized."""

        assert _env_truthy("NEBULA_MCP_LOCAL_INSECURE") is True

    @patch.dict("os.environ", {"NEBULA_MCP_LOCAL_INSECURE": "0"}, clear=True)
    def test_env_truthy_false_variants(self):
        """Falsy environment values should be rejected."""

        assert _env_truthy("NEBULA_MCP_LOCAL_INSECURE") is False

    @patch.dict("os.environ", {}, clear=True)
    def test_env_truthy_missing_var_is_false(self):
        """Missing env values should default to false."""

        assert _env_truthy("NEBULA_MCP_LOCAL_INSECURE") is False

    @patch.dict("os.environ", {"NEBULA_MCP_LOCAL_AGENT_NAME": " custom-agent "}, clear=True)
    def test_local_insecure_agent_name_uses_env_value(self):
        """Agent-name helper should trim and use override values."""

        assert _local_insecure_agent_name() == "custom-agent"

    @patch.dict("os.environ", {}, clear=True)
    def test_local_insecure_agent_name_defaults(self):
        """Agent-name helper should fallback to default when unset."""

        assert _local_insecure_agent_name() == "local-dev-agent"

    @patch.dict("os.environ", {"NEBULA_MCP_LOCAL_AGENT_NAME": "   "}, clear=True)
    def test_local_insecure_agent_name_whitespace_defaults(self):
        """Whitespace-only overrides should fallback to default agent name."""

        assert _local_insecure_agent_name() == "local-dev-agent"

    @patch.dict("os.environ", {"NEBULA_MCP_LOCAL_AGENT_NAME": "local-existing"}, clear=True)
    async def test_get_or_create_local_insecure_agent_returns_existing(self, mock_pool, mock_enums):
        """Existing local agent should be returned without creation."""

        mock_pool.fetchrow.return_value = {"id": uuid4(), "name": "local-existing"}
        agent = await _get_or_create_local_insecure_agent(mock_pool, mock_enums)
        assert agent["name"] == "local-existing"

    @patch.dict("os.environ", {"NEBULA_MCP_LOCAL_AGENT_NAME": "local-new"}, clear=True)
    async def test_get_or_create_local_insecure_agent_requires_scope(self, mock_pool, mock_enums):
        """Missing all known scopes should fail with explicit error."""

        mock_pool.fetchrow.side_effect = [None]
        mock_enums.scopes.name_to_id = {}
        with pytest.raises(ValueError, match="at least one valid scope"):
            await _get_or_create_local_insecure_agent(mock_pool, mock_enums)

    @patch.dict("os.environ", {"NEBULA_MCP_LOCAL_AGENT_NAME": "local-new"}, clear=True)
    async def test_get_or_create_local_insecure_agent_requires_active_status(
        self, mock_pool, mock_enums
    ):
        """Missing active status enum should fail clearly."""

        mock_pool.fetchrow.side_effect = [None]
        mock_enums.statuses.name_to_id = {}
        with pytest.raises(ValueError, match="active status enum"):
            await _get_or_create_local_insecure_agent(mock_pool, mock_enums)

    @patch.dict("os.environ", {"NEBULA_MCP_LOCAL_AGENT_NAME": "local-new"}, clear=True)
    async def test_get_or_create_local_insecure_agent_create_failure_raises(
        self, mock_pool, mock_enums
    ):
        """Creation failure should raise explicit local-insecure error."""

        mock_pool.fetchrow.side_effect = [None, None]
        with pytest.raises(ValueError, match="Failed to create local insecure agent"):
            await _get_or_create_local_insecure_agent(mock_pool, mock_enums)

    @patch.dict("os.environ", {"NEBULA_MCP_LOCAL_AGENT_NAME": "local-new"}, clear=True)
    async def test_get_or_create_local_insecure_agent_create_success(self, mock_pool, mock_enums):
        """Missing local agent should be created and returned."""

        created = {"id": uuid4(), "name": "local-new", "requires_approval": False}
        mock_pool.fetchrow.side_effect = [None, created]

        result = await _get_or_create_local_insecure_agent(mock_pool, mock_enums)

        assert result["name"] == "local-new"
        assert result["requires_approval"] is False


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

    @patch("nebula_mcp.helpers.create_approval_request", return_value=None)
    async def test_untrusted_with_missing_approval_id_returns_none_id(
        self, _mock_create_approval, mock_pool, mock_untrusted_agent
    ):
        """Approval response should tolerate missing helper return rows."""

        result = await maybe_require_approval(
            mock_pool,
            mock_untrusted_agent,
            "update_entity",
            {"name": "x"},
        )

        assert result is not None
        assert result["approval_request_id"] is None
        assert result["requested_action"] == "update_entity"
