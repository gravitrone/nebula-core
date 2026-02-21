"""Deterministic semantic scoring helpers used by API and MCP search flows."""

# Standard Library
import hashlib
import math
import re
from typing import Any

SEMANTIC_DIMENSIONS = 256
MIN_SEMANTIC_SCORE = 0.05

STOP_WORDS = {
    "a",
    "an",
    "and",
    "are",
    "as",
    "at",
    "be",
    "by",
    "for",
    "from",
    "in",
    "is",
    "it",
    "of",
    "on",
    "or",
    "that",
    "the",
    "to",
    "with",
}

SEMANTIC_EXPANSIONS = {
    "agent": {"assistant", "automation", "orchestrator"},
    "agents": {"assistant", "automation", "orchestrators"},
    "ai": {"artificial", "intelligence", "machine", "learning"},
    "context": {"memory", "facts", "notes", "history"},
    "memory": {"context", "facts", "history"},
    "prompt": {"instruction", "context"},
    "retrieval": {"search", "lookup", "query"},
    "search": {"lookup", "retrieval", "find"},
    "task": {"job", "work", "todo"},
    "jobs": {"tasks", "work", "queue"},
    "relationship": {"link", "connection", "edge"},
}


def _tokenize(text: str) -> list[str]:
    """Handle tokenize.

    Args:
        text: Input parameter for _tokenize.

    Returns:
        Result value from the operation.
    """

    normalized = re.sub(r"[^a-z0-9\s]+", " ", text.lower())
    parts = [p for p in normalized.split() if p and p not in STOP_WORDS]
    out: list[str] = []
    for part in parts:
        out.append(part)
        if len(part) > 3 and part.endswith("s"):
            out.append(part[:-1])
    return out


def _expanded_tokens(text: str) -> list[str]:
    """Handle expanded tokens.

    Args:
        text: Input parameter for _expanded_tokens.

    Returns:
        Result value from the operation.
    """

    tokens = _tokenize(text)
    expanded = list(tokens)
    for token in tokens:
        expanded.extend(SEMANTIC_EXPANSIONS.get(token, set()))
    return expanded


def _hash_index(token: str, dims: int) -> tuple[int, float]:
    """Handle hash index.

    Args:
        token: Input parameter for _hash_index.
        dims: Input parameter for _hash_index.

    Returns:
        Result value from the operation.
    """

    digest = hashlib.blake2b(token.encode("utf-8"), digest_size=8).digest()
    idx = int.from_bytes(digest[:4], "big") % dims
    sign = 1.0 if digest[4] & 1 else -1.0
    return idx, sign


def _embed_text(text: str, dims: int = SEMANTIC_DIMENSIONS) -> list[float]:
    """Handle embed text.

    Args:
        text: Input parameter for _embed_text.
        dims: Input parameter for _embed_text.

    Returns:
        Result value from the operation.
    """

    vec = [0.0] * dims
    tokens = _expanded_tokens(text)
    for token in tokens:
        idx, sign = _hash_index(token, dims)
        weight = 1.0
        if len(token) >= 8:
            weight += 0.2
        vec[idx] += sign * weight

    # Add character trigrams for typo resistance.
    normalized = re.sub(r"\s+", " ", text.lower()).strip()
    for i in range(max(0, len(normalized) - 2)):
        trigram = normalized[i : i + 3]
        if " " in trigram:
            continue
        idx, sign = _hash_index(f"tri:{trigram}", dims)
        vec[idx] += 0.2 * sign

    norm = math.sqrt(sum(v * v for v in vec))
    if norm == 0.0:
        return vec
    return [v / norm for v in vec]


def semantic_similarity(query: str, text: str) -> float:
    """Compute cosine similarity between deterministic query and text embeddings."""

    query_vec = _embed_text(query)
    text_vec = _embed_text(text)
    return float(sum(a * b for a, b in zip(query_vec, text_vec, strict=True)))


def rank_semantic_candidates(
    query: str,
    candidates: list[dict[str, Any]],
    *,
    text_key: str = "text",
    limit: int = 20,
    min_score: float = MIN_SEMANTIC_SCORE,
) -> list[dict[str, Any]]:
    """Rank candidates by semantic similarity and return top matches."""

    scored: list[dict[str, Any]] = []
    for item in candidates:
        text = str(item.get(text_key, "")).strip()
        if not text:
            continue
        score = semantic_similarity(query, text)
        if score < min_score:
            continue
        row = dict(item)
        row["score"] = round(score, 4)
        scored.append(row)

    scored.sort(
        key=lambda r: (
            -float(r["score"]),
            str(r.get("kind", "")),
            str(r.get("id", "")),
        )
    )
    return scored[:limit]
