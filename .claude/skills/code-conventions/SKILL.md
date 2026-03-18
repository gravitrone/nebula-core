---
name: code-conventions
description: Code style and conventions for nebula-core. Use this skill when writing, reviewing, or modifying any Python server code or Go CLI code in this repository. Applies to all code generation, refactoring, and review tasks.
---

# Code Conventions

Follow these conventions when writing or modifying code in this repository.

## Python (Server)

### Module Docstrings

Every Python file must start with a module-level docstring describing its purpose:

```python
"""Context API routes."""
```

One-liner, concise, describes what the file contains.

### Imports

Import sorting is handled automatically by ruff (isort). Just write your imports and ruff will group them into standard library, third-party, and local sections.

```python
import json
from pathlib import Path

from asyncpg import Pool
from pydantic import BaseModel

from nebula_mcp.db import get_pool
from nebula_mcp.models import CreateEntityInput
```

Do not add section comments (`# Standard Library`, etc.) - ruff handles the ordering.

### Section Separators

Use dashed comments for major code sections. Section names use proper capitalization:

```python

# --- Admin Tools ---


# --- Entity Tools ---


# --- Type Aliases ---

```

### Inline Comments

Write clean text with no brackets or decorations. Use proper capitalization:

```python
# Validate enums first (fail fast)
# Privacy check
# Calculate scope intersection before filtering
```

### Docstrings

Use Google-style with summary on the opening line. Proper capitalization:

```python
def create_entity(name: str, type_id: int) -> dict:
    """Create an entity in the database.

    Args:
        name: Display name for the entity.
        type_id: Entity type ID.

    Returns:
        Created entity row as dict.

    Raises:
        ValueError: If validation fails.
    """
```

Rules:
- Opening `"""` and summary on the same line
- Blank line before Args/Returns/Raises sections
- Include only relevant sections (omit empty ones)
- Simple functions can use one-liners: `"""Reload enum cache."""`
- Always leave one blank line after the closing `"""`

### SQL Queries

All SQL lives in `.sql` files under `src/queries/`, organized by domain:

```
queries/
├── entities/
│   ├── create.sql
│   ├── get.sql
│   └── query.sql
├── context/
│   ├── create.sql
│   ├── by_owner.sql
│   └── query.sql
└── jobs/
    ├── create.sql
    └── update.sql
```

Access via `QueryLoader`:

```python
from .query_loader import QueryLoader

QUERIES = QueryLoader(Path(__file__).resolve().parents[1] / "queries")

row = await pool.fetchrow(QUERIES["entities/get"], entity_id)
```

Rules:
- Every `.sql` file starts with `-- <short description>`
- No inline SQL except PostgreSQL session commands (`SET`, `RESET`)

### Mid-Function Imports

Only use when avoiding circular imports:

```python
async def execute_action(pool, enums, details):
    """Execute action."""

    # Import here to avoid circular dependency
    from .models import ActionInput
```

Move all other imports to the file top.

## Go (CLI)

### Package Comments

Every Go package should have a comment if it exports symbols.

### Imports

Use standard goimports grouping (stdlib, external, local) with blank line separators:

```go
import (
    "encoding/json"
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

    "github.com/gravitrone/nebula-core/cli/internal/api"
    "github.com/gravitrone/nebula-core/cli/internal/ui/components"
)
```

### Section Separators

Same pattern as Python:

```go
// --- Messages ---

// --- View ---

// --- Update ---
```

### Comments

Use standard Go doc comment conventions. Proper capitalization. Exported functions must have comments starting with the function name:

```go
// CreateEntity sends a create entity request to the API.
func (c *Client) CreateEntity(input CreateEntityInput) (*Entity, error) {
```

### Struct Tags

Always use `json` tags on API types:

```go
type Entity struct {
    ID     string   `json:"id"`
    Name   string   `json:"name"`
    Type   string   `json:"type"`
    Status string   `json:"status"`
    Tags   []string `json:"tags"`
}
```

### Error Handling

Return errors, don't panic. Use `fmt.Errorf` with `%w` for wrapping:

```go
if err != nil {
    return fmt.Errorf("failed to create entity: %w", err)
}
```

## General

### Commits

Conventional commits: `type(scope): description`

Types: `feat`, `fix`, `refactor`, `docs`, `infra`, `test`, `chore`

Example: `feat(context): add list-by-owner endpoint`

No co-author tags.

### Comments

Only add comments where logic is not self-evident. Do not over-comment obvious code. Use proper capitalization in all comments and docstrings.

### Engineering Philosophy

Do not over-engineer. Keep solutions simple and direct. Add abstraction only when a clear repeated pattern emerges, not preemptively.
