"""add job_type column to jobs

Revision ID: 305d65f8b1b2
Revises: 9764083ff366
Create Date: 2026-03-29 22:11:20.568165

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = '305d65f8b1b2'
down_revision: Union[str, Sequence[str], None] = '9764083ff366'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Upgrade schema."""
    op.add_column('jobs', sa.Column('job_type', sa.Text(), nullable=True))


def downgrade() -> None:
    """Downgrade schema."""
    op.drop_column('jobs', 'job_type')
