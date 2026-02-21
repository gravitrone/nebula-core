"""Schema contract API routes."""

# Standard Library
from typing import Any

# Third-Party
from fastapi import APIRouter, Depends, Request

# Local
from nebula_api.auth import require_auth
from nebula_api.response import success
from nebula_mcp.schema import load_schema_contract

router = APIRouter()


@router.get("/")
async def get_schema(
    request: Request,
    auth: dict = Depends(require_auth),
) -> dict[str, Any]:
    """Return the canonical schema contract for agents and clients.

    Args:
        request: FastAPI request.
        auth: Auth context.

    Returns:
        API response with active taxonomy and core enum constraints.
    """

    _ = auth
    pool = request.app.state.pool
    contract = await load_schema_contract(pool)
    return success(contract)
