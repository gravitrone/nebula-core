"""Entity model (person, project, organization, tool, document)."""

import uuid
from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, Text, text
from sqlalchemy.dialects.postgresql import ARRAY, UUID
from sqlalchemy.orm import Mapped, mapped_column, relationship

from nebula_models.base import Base, IDMixin, TimestampMixin


class Entity(Base, IDMixin, TimestampMixin):
    """Core entity in the knowledge graph."""

    __tablename__ = "entities"

    privacy_scope_ids: Mapped[list[uuid.UUID]] = mapped_column(
        ARRAY(UUID(as_uuid=True)), server_default=text("'{}'"), nullable=False
    )
    name: Mapped[str] = mapped_column(Text, nullable=False)
    type_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("entity_types.id"), nullable=False
    )
    status_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("statuses.id")
    )
    status_changed_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    status_reason: Mapped[str | None] = mapped_column(Text)
    tags: Mapped[list[str]] = mapped_column(
        ARRAY(Text), server_default=text("'{}'"), nullable=False
    )
    # No metadata column - replaced by context-of relationships (migration 021)
    source_path: Mapped[str | None] = mapped_column(Text)

    entity_type = relationship("EntityType", lazy="joined")
    status = relationship("Status", lazy="joined")
