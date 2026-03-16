"""Taxonomy tables: entity_types, log_types, privacy_scopes, relationship_types, statuses."""

from sqlalchemy import Boolean, String, Text, text
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy.orm import Mapped, mapped_column

from nebula_models.base import Base, IDMixin, TimestampMixin


class EntityType(Base, IDMixin, TimestampMixin):
    """Entity type taxonomy (person, project, organization, etc.)."""

    __tablename__ = "entity_types"

    name: Mapped[str] = mapped_column(Text, nullable=False, unique=True)
    description: Mapped[str | None] = mapped_column(Text)
    is_builtin: Mapped[bool] = mapped_column(Boolean, server_default=text("false"), nullable=False)
    is_active: Mapped[bool] = mapped_column(Boolean, server_default=text("true"), nullable=False)
    metadata_: Mapped[dict | None] = mapped_column("metadata", JSONB, server_default=text("'{}'"))
    value_schema: Mapped[dict | None] = mapped_column(JSONB)


class LogType(Base, IDMixin, TimestampMixin):
    """Log type taxonomy (event, metric, note, etc.)."""

    __tablename__ = "log_types"

    name: Mapped[str] = mapped_column(Text, nullable=False, unique=True)
    description: Mapped[str | None] = mapped_column(Text)
    is_builtin: Mapped[bool] = mapped_column(Boolean, server_default=text("false"), nullable=False)
    is_active: Mapped[bool] = mapped_column(Boolean, server_default=text("true"), nullable=False)
    metadata_: Mapped[dict | None] = mapped_column("metadata", JSONB, server_default=text("'{}'"))
    value_schema: Mapped[dict | None] = mapped_column(JSONB)


class PrivacyScope(Base, IDMixin, TimestampMixin):
    """Privacy scope taxonomy (public, private, sensitive, admin)."""

    __tablename__ = "privacy_scopes"

    name: Mapped[str] = mapped_column(Text, nullable=False, unique=True)
    description: Mapped[str | None] = mapped_column(Text)
    is_builtin: Mapped[bool] = mapped_column(Boolean, server_default=text("false"), nullable=False)
    is_active: Mapped[bool] = mapped_column(Boolean, server_default=text("true"), nullable=False)
    metadata_: Mapped[dict | None] = mapped_column("metadata", JSONB, server_default=text("'{}'"))


class RelationshipType(Base, IDMixin, TimestampMixin):
    """Relationship type taxonomy (related-to, owns, context-of, etc.)."""

    __tablename__ = "relationship_types"

    name: Mapped[str] = mapped_column(Text, nullable=False, unique=True)
    description: Mapped[str | None] = mapped_column(Text)
    is_symmetric: Mapped[bool] = mapped_column(
        Boolean, server_default=text("false"), nullable=False
    )
    is_builtin: Mapped[bool] = mapped_column(Boolean, server_default=text("false"), nullable=False)
    is_active: Mapped[bool] = mapped_column(Boolean, server_default=text("true"), nullable=False)
    metadata_: Mapped[dict | None] = mapped_column("metadata", JSONB, server_default=text("'{}'"))


class Status(Base, IDMixin, TimestampMixin):
    """Status taxonomy (active, in-progress, planning, etc.)."""

    __tablename__ = "statuses"

    name: Mapped[str] = mapped_column(Text, nullable=False, unique=True)
    description: Mapped[str | None] = mapped_column(Text)
    category: Mapped[str] = mapped_column(String, server_default=text("'active'"), nullable=False)
    is_builtin: Mapped[bool] = mapped_column(Boolean, server_default=text("false"), nullable=False)
    is_active: Mapped[bool] = mapped_column(Boolean, server_default=text("true"), nullable=False)
    metadata_: Mapped[dict | None] = mapped_column("metadata", JSONB, server_default=text("'{}'"))
