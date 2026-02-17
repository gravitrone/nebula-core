"""Unit tests for enum registry and validators."""

# Standard Library
from uuid import UUID

import pytest

from nebula_mcp.enums import (
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
