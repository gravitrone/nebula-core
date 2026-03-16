"""Unit tests for exports route helper and snapshot edge branches."""

# Standard Library
from types import SimpleNamespace
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest

# Local
from nebula_api.routes.exports import (
    _job_visible,
    _resolve_scope_ids,
    _visible_scope_names,
    export_snapshot,
)


pytestmark = pytest.mark.unit


def _request(pool, enums):
    """Build a minimal request with app state."""

    return SimpleNamespace(app=SimpleNamespace(state=SimpleNamespace(pool=pool, enums=enums)))


def test_resolve_scope_ids_explicit_scope_subset_success(mock_enums):
    """Explicit scopes should resolve to UUID ids when subset is valid."""

    public_id = mock_enums.scopes.name_to_id["public"]
    private_id = mock_enums.scopes.name_to_id["private"]
    auth = {"scopes": [public_id, private_id]}

    resolved = _resolve_scope_ids(["public"], auth, mock_enums)

    assert resolved == [public_id]


def test_visible_scope_names_admin_returns_all_scope_names(mock_enums):
    """Admin callers should see the full sorted scope-name list."""

    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}
    names = _visible_scope_names(auth, mock_enums, None)
    assert names == sorted(mock_enums.scopes.name_to_id.keys())


@pytest.mark.asyncio
async def test_job_visible_admin_short_circuits(mock_enums):
    """Admin callers should bypass job lookups."""

    pool = SimpleNamespace(fetchrow=AsyncMock())
    auth = {"scopes": [mock_enums.scopes.name_to_id["admin"]]}

    visible = await _job_visible(pool, auth, mock_enums, "2026Q1-ABCD")

    assert visible is True
    pool.fetchrow.assert_not_awaited()


@pytest.mark.asyncio
async def test_job_visible_missing_job_returns_false(mock_enums):
    """Missing jobs should be hidden from non-admin callers."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value=None))
    auth = {"scopes": [mock_enums.scopes.name_to_id["public"]]}

    visible = await _job_visible(pool, auth, mock_enums, "2026Q1-ABCD")

    assert visible is False


@pytest.mark.asyncio
async def test_job_visible_empty_job_scope_returns_true(mock_enums):
    """Jobs without scopes should be visible to non-admin callers."""

    pool = SimpleNamespace(fetchrow=AsyncMock(return_value={"privacy_scope_ids": []}))
    auth = {"scopes": [mock_enums.scopes.name_to_id["public"]]}

    visible = await _job_visible(pool, auth, mock_enums, "2026Q1-ABCD")

    assert visible is True


@pytest.mark.asyncio
async def test_export_snapshot_hides_target_job_and_keeps_context_rows(monkeypatch, mock_enums):
    """Snapshot export should skip hidden target jobs and include context rows."""

    public_id = mock_enums.scopes.name_to_id["public"]
    auth = {"scopes": [public_id]}
    context_id = str(uuid4())
    relationship_row = {
        "id": str(uuid4()),
        "source_type": "entity",
        "source_id": str(uuid4()),
        "target_type": "job",
        "target_id": "2026Q1-ABCD",
        "properties": {"context_segments": [{"text": "secret", "scopes": ["private"]}]},
    }
    pool = SimpleNamespace(
        fetch=AsyncMock(
            side_effect=[
                [{"id": str(uuid4())}],  # entities rows
                [{"id": context_id}],  # context rows
                [relationship_row],  # relationships rows
                [],  # jobs rows
            ]
        )
    )

    monkeypatch.setattr(
        "nebula_api.routes.exports._job_visible",
        AsyncMock(return_value=False),
    )

    result = await export_snapshot(
        _request(pool, mock_enums),
        auth=auth,
        format="json",
    )

    assert result["data"]["context"][0]["id"] == context_id
    assert result["data"]["relationships"] == []
