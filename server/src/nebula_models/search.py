"""Semantic search model (embedding-based search index)."""

import uuid
from datetime import datetime

from sqlalchemy import DateTime, Integer, Text, text
from sqlalchemy.dialects.postgresql import ARRAY, UUID
from sqlalchemy.orm import Mapped, mapped_column

from nebula_models.base import Base, IDMixin


class SemanticSearch(Base, IDMixin):
    """Semantic search index entry linking embeddings to source records."""

    __tablename__ = "semantic_search"

    source_type: Mapped[str] = mapped_column(Text, nullable=False)
    source_id: Mapped[str] = mapped_column(Text, nullable=False)
    segment_index: Mapped[int | None] = mapped_column(Integer)
    # embedding: vector(1536) - handled by pgvector, not modeled here
    scopes: Mapped[list[uuid.UUID]] = mapped_column(
        ARRAY(UUID(as_uuid=True)), server_default=text("'{}'"), nullable=False
    )
    notes: Mapped[str | None] = mapped_column(Text, server_default=text("''"))
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=text("now()"), nullable=False
    )
