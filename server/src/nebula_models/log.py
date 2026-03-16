"""Log model (timestamped entries with type, value, tags)."""

import uuid
from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, Text, text
from sqlalchemy.dialects.postgresql import ARRAY, JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column, relationship

from nebula_models.base import Base, IDMixin, TimestampMixin


class Log(Base, IDMixin, TimestampMixin):
    """Timestamped log entry linked to entities/jobs."""

    __tablename__ = "logs"

    log_type_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("log_types.id")
    )
    timestamp: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=text("now()"), nullable=False
    )
    value: Mapped[dict | None] = mapped_column(JSONB, server_default=text("'{}'"))
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

    log_type = relationship("LogType", lazy="joined")
    status = relationship("Status", lazy="joined")
