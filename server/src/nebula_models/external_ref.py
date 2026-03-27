"""External reference model (links to external systems like Linear, GitHub, etc.)."""

from sqlalchemy import Text, text
from sqlalchemy.orm import Mapped, mapped_column

from nebula_models.base import Base, IDMixin, TimestampMixin


class ExternalRef(Base, IDMixin, TimestampMixin):
    """Links a nebula node to an external system record."""

    __tablename__ = "external_refs"

    node_type: Mapped[str] = mapped_column(Text, nullable=False)
    node_id: Mapped[str] = mapped_column(Text, nullable=False)
    system: Mapped[str] = mapped_column(Text, nullable=False)
    external_id: Mapped[str] = mapped_column(Text, nullable=False)
    url: Mapped[str | None] = mapped_column(Text)
    notes: Mapped[str] = mapped_column(Text, server_default=text("''"), nullable=False)
