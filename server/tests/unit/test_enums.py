"""Unit tests for enum registry and validators."""

# Standard Library
from unittest.mock import AsyncMock
from uuid import UUID

import pytest

from nebula_mcp.enums import (
    _load_section,
    load_enums,
    require_entity_type,
    require_log_type,
    require_relationship_type,
    require_scopes,
    require_status,
)

pytestmark = pytest.mark.unit


# --- EnumSection Bidirectional ---


class TestEnumSection:
    """Tests for EnumSection bidirectional consistency."""

    def test_bidirectional_consistency(self, mock_enums):
        """Verify name_to_id and id_to_name are inverse mappings."""

        section = mock_enums.statuses
        for name, uid in section.name_to_id.items():
            assert section.id_to_name[uid] == name


# --- require_status ---


class TestRequireStatus:
    """Tests for the require_status validator."""

    def test_valid_status(self, mock_enums):
        """Return UUID for a known status name."""

        result = require_status("active", mock_enums)
        assert isinstance(result, UUID)

    def test_unknown_status_raises(self, mock_enums):
        """Raise ValueError for an unknown status name."""

        with pytest.raises(ValueError, match="Unknown status"):
            require_status("nonexistent", mock_enums)

    def test_empty_status_raises(self, mock_enums):
        """Raise ValueError for an empty status string."""

        with pytest.raises(ValueError, match="Status required"):
            require_status("", mock_enums)

    def test_none_status_raises(self, mock_enums):
        """Raise ValueError for None status input."""

        with pytest.raises(ValueError, match="Status required"):
            require_status(None, mock_enums)  # type: ignore[arg-type]

    def test_archived_alias_maps_to_terminal_status(self, mock_enums):
        """Map archived alias to a known terminal status UUID."""

        result = require_status("archived", mock_enums)
        assert result in mock_enums.statuses.id_to_name
        assert mock_enums.statuses.id_to_name[result] in {
            "inactive",
            "completed",
            "abandoned",
            "deleted",
            "replaced",
            "on-hold",
        }

    def test_status_normalizes_case_and_whitespace(self, mock_enums):
        """Status lookup should normalize whitespace and casing."""

        assert require_status(" Active ", mock_enums) == require_status(
            "active", mock_enums
        )

    def test_whitespace_only_status_raises_unknown_status(self, mock_enums):
        """Whitespace-only status strings should fail with unknown-status error."""

        with pytest.raises(ValueError, match="Unknown status"):
            require_status("   ", mock_enums)

    def test_archived_alias_without_terminal_candidates_raises(self, mock_enums):
        """Archived alias should error when no terminal status exists."""

        for candidate in (
            "inactive",
            "completed",
            "abandoned",
            "deleted",
            "replaced",
            "on-hold",
        ):
            mock_enums.statuses.name_to_id.pop(candidate, None)

        with pytest.raises(ValueError, match="Unknown status: archived"):
            require_status("archived", mock_enums)

    def test_archived_alias_falls_back_to_next_available_terminal(self, mock_enums):
        """Archived alias should use the next available terminal candidate."""

        mock_enums.statuses.name_to_id.pop("inactive", None)

        result = require_status("archived", mock_enums)
        assert result == mock_enums.statuses.name_to_id["completed"]

    def test_archived_alias_is_case_insensitive(self, mock_enums):
        """Archived alias mapping should work for uppercase variants."""

        result = require_status("ARCHIVED", mock_enums)
        assert result in mock_enums.statuses.id_to_name


# --- require_entity_type ---


class TestRequireEntityType:
    """Tests for the require_entity_type validator."""

    def test_valid_entity_type(self, mock_enums):
        """Return UUID for a known entity type name."""

        result = require_entity_type("person", mock_enums)
        assert isinstance(result, UUID)

    def test_unknown_entity_type_raises(self, mock_enums):
        """Raise ValueError for an unknown entity type name."""

        with pytest.raises(ValueError, match="Unknown entity type"):
            require_entity_type("alien", mock_enums)

    def test_empty_entity_type_raises(self, mock_enums):
        """Raise ValueError for an empty entity type string."""

        with pytest.raises(ValueError, match="Entity type required"):
            require_entity_type("", mock_enums)

    def test_none_entity_type_raises(self, mock_enums):
        """Raise ValueError for None entity type input."""

        with pytest.raises(ValueError, match="Entity type required"):
            require_entity_type(None, mock_enums)  # type: ignore[arg-type]

    def test_entity_type_case_sensitive_unknown_raises(self, mock_enums):
        """Entity type validator should reject mismatched case."""

        with pytest.raises(ValueError, match="Unknown entity type"):
            require_entity_type("Person", mock_enums)


# --- require_relationship_type ---


class TestRequireRelationshipType:
    """Tests for the require_relationship_type validator."""

    def test_valid_relationship_type(self, mock_enums):
        """Return UUID for a known relationship type name."""

        result = require_relationship_type("related-to", mock_enums)
        assert isinstance(result, UUID)

    def test_unknown_relationship_type_raises(self, mock_enums):
        """Raise ValueError for an unknown relationship type name."""

        with pytest.raises(ValueError, match="Unknown relationship type"):
            require_relationship_type("enemies-with", mock_enums)

    def test_empty_relationship_type_raises(self, mock_enums):
        """Raise ValueError for empty relationship type input."""

        with pytest.raises(ValueError, match="Relationship type required"):
            require_relationship_type("", mock_enums)

    def test_none_relationship_type_raises(self, mock_enums):
        """Raise ValueError for None relationship type input."""

        with pytest.raises(ValueError, match="Relationship type required"):
            require_relationship_type(None, mock_enums)  # type: ignore[arg-type]


# --- require_scopes ---


class TestRequireScopes:
    """Tests for the require_scopes validator."""

    def test_valid_scope_list(self, mock_enums):
        """Return list of UUIDs for known scope names."""

        result = require_scopes(["public", "private"], mock_enums)
        assert len(result) == 2
        assert all(isinstance(uid, UUID) for uid in result)

    def test_empty_list_raises(self, mock_enums):
        """Raise ValueError for an empty scope list."""

        with pytest.raises(ValueError, match="Scopes required"):
            require_scopes([], mock_enums)

    def test_one_unknown_in_list_raises(self, mock_enums):
        """Raise ValueError when one scope in the list is unknown."""

        with pytest.raises(ValueError, match="Unknown scope"):
            require_scopes(["public", "galactic"], mock_enums)

    def test_scope_list_preserves_order_and_duplicates(self, mock_enums):
        """Scope list should preserve caller order exactly."""

        result = require_scopes(["private", "public", "private"], mock_enums)
        assert result == [
            mock_enums.scopes.name_to_id["private"],
            mock_enums.scopes.name_to_id["public"],
            mock_enums.scopes.name_to_id["private"],
        ]

    def test_scope_values_are_not_trimmed(self, mock_enums):
        """Whitespace-padded scopes should fail without implicit trimming."""

        with pytest.raises(ValueError, match="Unknown scope"):
            require_scopes([" public "], mock_enums)

    def test_empty_scope_value_raises_unknown_scope(self, mock_enums):
        """Empty scope values should fail with a clear unknown-scope error."""

        with pytest.raises(ValueError, match="Unknown scope"):
            require_scopes([""], mock_enums)

    def test_none_scope_value_raises_unknown_scope(self, mock_enums):
        """None values should fail scope lookup with clear unknown-scope error."""

        with pytest.raises(ValueError, match="Unknown scope: None"):
            require_scopes(["public", None], mock_enums)  # type: ignore[list-item]


# --- require_log_type ---


class TestRequireLogType:
    """Tests for the require_log_type validator."""

    def test_valid_log_type(self, mock_enums):
        """Return UUID for a known log type name."""

        result = require_log_type("note", mock_enums)
        assert isinstance(result, UUID)

    def test_empty_log_type_raises(self, mock_enums):
        """Raise ValueError for an empty log type string."""

        with pytest.raises(ValueError, match="Log type required"):
            require_log_type("", mock_enums)

    def test_none_log_type_raises(self, mock_enums):
        """Raise ValueError for None log type input."""

        with pytest.raises(ValueError, match="Log type required"):
            require_log_type(None, mock_enums)  # type: ignore[arg-type]

    def test_unknown_log_type_raises(self, mock_enums):
        """Unknown log type values should raise clear validation errors."""

        with pytest.raises(ValueError, match="Unknown log type"):
            require_log_type("incident", mock_enums)


class TestLoadEnums:
    """Tests for async enum registry loading."""

    @pytest.mark.asyncio
    async def test_load_section_builds_bidirectional_maps(self):
        """_load_section should build name and id maps from query rows."""

        active_id = UUID(int=11)
        inactive_id = UUID(int=12)
        pool = AsyncMock()
        pool.fetch = AsyncMock(
            return_value=[
                {"name": "active", "id": active_id},
                {"name": "inactive", "id": inactive_id},
            ]
        )

        section = await _load_section(pool, "enums/statuses")

        assert section.name_to_id == {"active": active_id, "inactive": inactive_id}
        assert section.id_to_name == {active_id: "active", inactive_id: "inactive"}
        pool.fetch.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_load_section_handles_empty_rows(self):
        """_load_section should return empty maps when query has no rows."""

        pool = AsyncMock()
        pool.fetch = AsyncMock(return_value=[])

        section = await _load_section(pool, "enums/statuses")

        assert section.name_to_id == {}
        assert section.id_to_name == {}

    @pytest.mark.asyncio
    async def test_load_section_last_row_wins_on_duplicate_name_and_id(self):
        """Duplicate names or IDs should resolve to the last seen row."""

        first_id = UUID(int=21)
        second_id = UUID(int=22)
        pool = AsyncMock()
        pool.fetch = AsyncMock(
            return_value=[
                {"name": "active", "id": first_id},
                {"name": "active", "id": second_id},
                {"name": "inactive", "id": second_id},
            ]
        )

        section = await _load_section(pool, "enums/statuses")

        assert section.name_to_id["active"] == second_id
        assert section.id_to_name[second_id] == "inactive"

    @pytest.mark.asyncio
    async def test_load_enums_calls_all_sections_in_order(self, monkeypatch):
        """load_enums should request all five enum sections."""

        pool = object()
        calls = []

        async def _fake_load_section(p, query_name):
            calls.append((p, query_name))
            return type(
                "Section",
                (),
                {"name_to_id": {"x": UUID(int=1)}, "id_to_name": {UUID(int=1): "x"}},
            )()

        monkeypatch.setattr("nebula_mcp.enums._load_section", _fake_load_section)

        enums = await load_enums(pool)

        assert enums.statuses is not None
        assert enums.scopes is not None
        assert enums.relationship_types is not None
        assert enums.entity_types is not None
        assert enums.log_types is not None
        assert [q for _, q in calls] == [
            "enums/statuses",
            "enums/scopes",
            "enums/relationship_types",
            "enums/entity_types",
            "enums/log_types",
        ]

    @pytest.mark.asyncio
    async def test_load_enums_propagates_section_loading_error(self, monkeypatch):
        """load_enums should bubble errors from _load_section immediately."""

        calls = []

        async def _fake_load_section(_pool, query_name):
            calls.append(query_name)
            if query_name == "enums/scopes":
                raise RuntimeError("boom")
            return type(
                "Section",
                (),
                {"name_to_id": {"x": UUID(int=1)}, "id_to_name": {UUID(int=1): "x"}},
            )()

        monkeypatch.setattr("nebula_mcp.enums._load_section", _fake_load_section)

        with pytest.raises(RuntimeError, match="boom"):
            await load_enums(object())

        assert calls == ["enums/statuses", "enums/scopes"]
