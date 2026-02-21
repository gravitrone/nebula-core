"""Simple in-memory sliding window rate limiter."""

# Standard Library
import time
from collections import defaultdict
from collections.abc import Callable, Coroutine
from functools import wraps
from typing import Any

# Third-Party
from fastapi import HTTPException, Request

_buckets: dict[str, list[float]] = defaultdict(list)


def rate_limit(
    max_requests: int = 60, window: int = 60
) -> Callable[
    [Callable[..., Coroutine[Any, Any, Any]]],
    Callable[..., Coroutine[Any, Any, Any]],
]:
    """Create a rate limiting decorator.

    Args:
        max_requests: Maximum requests allowed in window.
        window: Time window in seconds.

    Returns:
        Decorator that enforces rate limiting.
    """

    def decorator(
        func: Callable[..., Coroutine[Any, Any, Any]],
    ) -> Callable[..., Coroutine[Any, Any, Any]]:
        """Handle decorator.

        Args:
            func: Input parameter for decorator.

        Returns:
            Result value from the operation.
        """

        @wraps(func)
        async def wrapper(*args: Any, request: Request, **kwargs: Any) -> Any:
            """Handle wrapper.

            Args:
                request: Input parameter for wrapper.
                *args: Input parameter for wrapper.
                **kwargs: Input parameter for wrapper.

            Returns:
                Result value from the operation.
            """

            key = request.headers.get("Authorization", request.client.host)
            now = time.time()
            _buckets[key] = [t for t in _buckets[key] if now - t < window]
            if len(_buckets[key]) >= max_requests:
                raise HTTPException(
                    status_code=429,
                    detail={
                        "error": {
                            "code": "RATE_LIMITED",
                            "message": f"Max {max_requests} requests per {window}s",
                        }
                    },
                )
            _buckets[key].append(now)
            return await func(*args, request=request, **kwargs)

        return wrapper

    return decorator
