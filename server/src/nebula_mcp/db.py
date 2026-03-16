"""Database connection pool and query utilities for Nebula MCP."""

import os
from pathlib import Path
from urllib.parse import quote_plus

import asyncpg
from asyncpg import Pool, Record

from .query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[1] / "queries")


def build_dsn() -> str:
    """Build a PostgreSQL DSN from environment variables.

    Returns:
        PostgreSQL connection string.

    Raises:
        ValueError: If POSTGRES_PASSWORD is not set.
    """

    host = os.getenv("POSTGRES_HOST", "localhost")
    port = os.getenv("POSTGRES_PORT", "6432")
    db_name = os.getenv("POSTGRES_DB", "nebula")
    user = os.getenv("POSTGRES_USER", "nebula")
    password = os.getenv("POSTGRES_PASSWORD")

    if not password:
        raise ValueError("POSTGRES_PASSWORD is required")

    safe_password = quote_plus(password)
    return f"postgresql://{user}:{safe_password}@{host}:{port}/{db_name}"


async def get_pool(
    *,
    min_size: int = 1,
    max_size: int = 10,
    command_timeout: int = 30,
) -> Pool:
    """Create and return an asyncpg connection pool.

    Args:
        min_size: Minimum pool size.
        max_size: Maximum pool size.
        command_timeout: Query timeout in seconds.

    Returns:
        Configured asyncpg connection pool.
    """

    dsn = build_dsn()
    try:
        return await asyncpg.create_pool(
            dsn=dsn,
            min_size=min_size,
            max_size=max_size,
            command_timeout=command_timeout,
        )
    except (OSError, asyncpg.PostgresError) as exc:
        message = str(exc)
        connection_error = isinstance(exc, ConnectionRefusedError) or (
            isinstance(exc, OSError) and exc.errno in {61, 111, 10061}
        )
        if (
            connection_error
            or "connection refused" in message.lower()
            or "could not connect to server" in message.lower()
            or "failed to connect" in message.lower()
        ):
            raise RuntimeError("Database connection failed. Is Docker running?") from exc
        raise


async def get_agent(pool: Pool, agent_name: str) -> Record | None:
    """Return an active agent row by name or None.

    Args:
        pool: Database connection pool.
        agent_name: Agent name to look up.

    Returns:
        Agent record if found and active, None otherwise.

    Raises:
        ValueError: If agent_name is empty.
    """

    if not agent_name:
        raise ValueError("agent_name required")
    return await pool.fetchrow(QUERIES["agents/get"], agent_name)
