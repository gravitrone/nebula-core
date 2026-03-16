"""File model (file metadata with URI, mime type, checksum)."""

import uuid

from sqlalchemy import BigInteger, ForeignKey, Text, text
from sqlalchemy.dialects.postgresql import ARRAY, JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column, relationship

from nebula_models.base import Base, IDMixin, TimestampMixin


class File(Base, IDMixin, TimestampMixin):
    """File metadata record (no blob storage, URI-based)."""

    __tablename__ = "files"

    filename: Mapped[str] = mapped_column(Text, nullable=False)
    uri: Mapped[str | None] = mapped_column(Text)
    file_path: Mapped[str | None] = mapped_column(Text)
    mime_type: Mapped[str | None] = mapped_column(Text)
    size_bytes: Mapped[int | None] = mapped_column(BigInteger)
    checksum: Mapped[str | None] = mapped_column(Text)
    privacy_scope_ids: Mapped[list[uuid.UUID]] = mapped_column(
        ARRAY(UUID(as_uuid=True)), server_default=text("'{}'"), nullable=False
    )
    status_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("statuses.id")
    )
    tags: Mapped[list[str]] = mapped_column(
        ARRAY(Text), server_default=text("'{}'"), nullable=False
    )
    metadata_: Mapped[dict | None] = mapped_column("metadata", JSONB, server_default=text("'{}'"))
    source_path: Mapped[str | None] = mapped_column(Text)

    status = relationship("Status", lazy="joined")
