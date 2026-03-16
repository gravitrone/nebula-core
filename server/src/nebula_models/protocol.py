"""Protocol model (operational instructions with versioning)."""

import uuid

from sqlalchemy import Boolean, ForeignKey, Integer, Text, text
from sqlalchemy.dialects.postgresql import ARRAY, JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column, relationship

from nebula_models.base import Base, IDMixin, TimestampMixin


class Protocol(Base, IDMixin, TimestampMixin):
    """Protocol document with version, content, and trust flag."""

    __tablename__ = "protocols"

    name: Mapped[str] = mapped_column(Text, nullable=False)
    title: Mapped[str | None] = mapped_column(Text)
    version: Mapped[int] = mapped_column(Integer, server_default=text("1"), nullable=False)
    content: Mapped[str | None] = mapped_column(Text)
    protocol_type: Mapped[str | None] = mapped_column(Text)
    applies_to: Mapped[list[str]] = mapped_column(
        ARRAY(Text), server_default=text("'{}'"), nullable=False
    )
    privacy_scope_ids: Mapped[list[uuid.UUID]] = mapped_column(
        ARRAY(UUID(as_uuid=True)), server_default=text("'{}'"), nullable=False
    )
    status_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("statuses.id")
    )
    tags: Mapped[list[str]] = mapped_column(
        ARRAY(Text), server_default=text("'{}'"), nullable=False
    )
    trusted: Mapped[bool] = mapped_column(Boolean, server_default=text("false"), nullable=False)
    metadata_: Mapped[dict | None] = mapped_column("metadata", JSONB, server_default=text("'{}'"))
    source_path: Mapped[str | None] = mapped_column(Text)

    status = relationship("Status", lazy="joined")
