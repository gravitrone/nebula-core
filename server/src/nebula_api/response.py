"""Standard API response helpers."""

from datetime import UTC, datetime
from typing import Any, NoReturn

from fastapi import HTTPException


def success(data: Any, **meta: Any) -> dict[str, Any]:
    """Create a success response envelope.

    Args:
        data: Response payload.
        **meta: Additional metadata fields.

    Returns:
        Dict with data and meta fields.
    """

    return {
        "data": data,
        "meta": {"timestamp": datetime.now(UTC).isoformat(), **meta},
    }


def paginated(data: list[Any], count: int, limit: int, offset: int) -> dict[str, Any]:
    """Create a paginated response envelope.

    Args:
        data: Response payload list.
        count: Total number of results.
        limit: Page size limit.
        offset: Pagination offset.

    Returns:
        Dict with data and pagination meta.
    """

    return {
        "data": data,
        "meta": {
            "count": count,
            "limit": limit,
            "offset": offset,
            "timestamp": datetime.now(UTC).isoformat(),
        },
    }


def api_error(code: str, message: str, status_code: int = 400) -> NoReturn:
    """Raise a standardized API error.

    Args:
        code: Error code string (e.g., VALIDATION_ERROR).
        message: Human-readable error message.
        status_code: HTTP status code.

    Raises:
        HTTPException: Always raises with error envelope.
    """

    raise HTTPException(
        status_code=status_code,
        detail={"error": {"code": code, "message": message}},
    )
