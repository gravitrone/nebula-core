"""Relationship model (polymorphic directed graph edges)."""

import uuid
from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, Text, UniqueConstraint, text
from sqlalchemy.dialects.postgresql import JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column, relationship

from nebula_models.base import Base, IDMixin


class Relationship(Base, IDMixin):
    """Polymorphic graph edge between any two node types."""

    __tablename__ = "relationships"
    __table_args__ = (
        UniqueConstraint(
            "source_type",
            "source_id",
            "target_type",
            "target_id",
            "type_id",
            name="relationships_unique_edge",
        ),
    )

    source_type: Mapped[str] = mapped_column(Text, nullable=False)
    source_id: Mapped[str] = mapped_column(Text, nullable=False)
    target_type: Mapped[str] = mapped_column(Text, nullable=False)
    target_id: Mapped[str] = mapped_column(Text, nullable=False)
    type_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("relationship_types.id"), nullable=False
    )
    status_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("statuses.id")
    )
    status_changed_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    properties: Mapped[dict | None] = mapped_column(JSONB, server_default=text("'{}'"))
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=text("now()"), nullable=False
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=text("now()"), nullable=False
    )

    relationship_type = relationship("RelationshipType", lazy="joined")
    status = relationship("Status", lazy="joined")
