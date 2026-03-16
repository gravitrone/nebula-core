"""Approval request model (pending writes for untrusted agents)."""

import uuid
from datetime import datetime

from sqlalchemy import DateTime, Text, text
from sqlalchemy.dialects.postgresql import JSONB, UUID
from sqlalchemy.orm import Mapped, mapped_column

from nebula_models.base import Base, IDMixin


class ApprovalRequest(Base, IDMixin):
    """Queued write request pending human review."""

    __tablename__ = "approval_requests"

    job_id: Mapped[str | None] = mapped_column(Text)
    request_type: Mapped[str] = mapped_column(Text, nullable=False)
    requested_by: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True))
    change_details: Mapped[dict | None] = mapped_column(JSONB, server_default=text("'{}'"))
    status: Mapped[str] = mapped_column(Text, server_default=text("'pending'"))
    reviewed_by: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True))
    reviewed_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    review_notes: Mapped[str | None] = mapped_column(Text)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=text("now()"), nullable=False
    )
    execution_error: Mapped[str | None] = mapped_column(Text)
    review_details: Mapped[dict] = mapped_column(JSONB, server_default=text("'{}'"), nullable=False)
