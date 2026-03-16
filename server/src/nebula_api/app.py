"""Nebula REST API application."""

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from dotenv import load_dotenv
from fastapi import FastAPI

from nebula_api.routes import (
    agents,
    approvals,
    audit,
    context,
    entities,
    exports,
    files,
    imports,
    jobs,
    keys,
    logs,
    protocols,
    relationships,
    schema,
    search,
    taxonomy,
)
from nebula_mcp.db import get_pool
from nebula_mcp.enums import load_enums

load_dotenv()


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    """Initialize and teardown shared application resources.

    Args:
        app: FastAPI application instance.

    Yields:
        None. Sets pool and enums on app.state.
    """

    pool = await get_pool(min_size=2, max_size=10)
    try:
        enums = await load_enums(pool)
        app.state.pool = pool
        app.state.enums = enums
        yield
    finally:
        await pool.close()


app = FastAPI(
    title="Nebula API",
    version="0.1.0",
    description="REST API for Nebula - Agent Context Control",
    lifespan=lifespan,
)

app.include_router(entities.router, prefix="/api/entities", tags=["Entities"])
app.include_router(audit.router, prefix="/api/audit", tags=["Audit"])
app.include_router(context.router, prefix="/api/context", tags=["Context"])
app.include_router(relationships.router, prefix="/api/relationships", tags=["Relationships"])
app.include_router(jobs.router, prefix="/api/jobs", tags=["Jobs"])
app.include_router(search.router, prefix="/api/search", tags=["Search"])
app.include_router(files.router, prefix="/api/files", tags=["Files"])
app.include_router(logs.router, prefix="/api/logs", tags=["Logs"])
app.include_router(imports.router, prefix="/api/import", tags=["Import"])
app.include_router(exports.router, prefix="/api/export", tags=["Export"])
app.include_router(protocols.router, prefix="/api/protocols", tags=["Protocols"])
app.include_router(approvals.router, prefix="/api/approvals", tags=["Approvals"])
app.include_router(agents.router, prefix="/api/agents", tags=["Agents"])
app.include_router(keys.router, prefix="/api/keys", tags=["Keys"])
app.include_router(taxonomy.router, prefix="/api/taxonomy", tags=["Taxonomy"])
app.include_router(schema.router, prefix="/api/schema", tags=["Schema"])


@app.get("/api/health")
async def health() -> dict[str, str]:
    """Health check endpoint.

    Returns:
        Dict with status ok.
    """

    return {"status": "ok"}
