"""Agent and enrollment session models."""

import uuid
from datetime import datetime

from sqlalchemy import Boolean, DateTime, ForeignKey, Text, text
from sqlalchemy.dialects.postgresql import ARRAY, UUID
from sqlalchemy.orm import Mapped, mapped_column, relationship

from nebula_models.base import Base, IDMixin, TimestampMixin


class Agent(Base, IDMixin, TimestampMixin):
    """AI agent with scopes, capabilities, and trust settings."""

    __tablename__ = "agents"

    name: Mapped[str] = mapped_column(Text, nullable=False, unique=True)
    description: Mapped[str | None] = mapped_column(Text)
    system_prompt: Mapped[str | None] = mapped_column(Text)
    scopes: Mapped[list[uuid.UUID]] = mapped_column(
        ARRAY(UUID(as_uuid=True)), server_default=text("'{}'"), nullable=False
    )
    capabilities: Mapped[list[str]] = mapped_column(
        ARRAY(Text), server_default=text("'{}'"), nullable=False
    )
    status_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("statuses.id")
    )
    notes: Mapped[str | None] = mapped_column(Text, server_default=text("''"))
    requires_approval: Mapped[bool] = mapped_column(
        Boolean, server_default=text("true"), nullable=False
    )

    status = relationship("Status", lazy="joined")


class AgentEnrollmentSession(Base, IDMixin, TimestampMixin):
    """Enrollment session for agent registration and approval flow."""

    __tablename__ = "agent_enrollment_sessions"

    agent_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("agents.id"), nullable=False
    )
    approval_request_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("approval_requests.id"), nullable=False
    )
    status: Mapped[str] = mapped_column(
        Text, server_default=text("'pending_approval'"), nullable=False
    )
    enrollment_token_hash: Mapped[str] = mapped_column(Text, nullable=False)
    requested_scope_ids: Mapped[list[uuid.UUID]] = mapped_column(
        ARRAY(UUID(as_uuid=True)), server_default=text("'{}'"), nullable=False
    )
    granted_scope_ids: Mapped[list[uuid.UUID] | None] = mapped_column(
        ARRAY(UUID(as_uuid=True)), server_default=text("'{}'")
    )
    requested_requires_approval: Mapped[bool] = mapped_column(
        Boolean, server_default=text("true"), nullable=False
    )
    granted_requires_approval: Mapped[bool | None] = mapped_column(Boolean)
    rejected_reason: Mapped[str | None] = mapped_column(Text)
    approved_by: Mapped[uuid.UUID | None] = mapped_column(UUID(as_uuid=True))
    approved_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    redeemed_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    expires_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)

    agent = relationship("Agent", lazy="joined")
    approval_request = relationship("ApprovalRequest", lazy="joined")
