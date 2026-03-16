# Nebula Core

Git for agent context. Secure system-of-record where AI agents coordinate shared memory through scopes, approvals, audit trails, and rollback. Built by Gravitrone.

## Critical Rules

- ALWAYS use QueryLoader for SQL. All queries in `.sql` files under `server/src/queries/`. No inline SQL in Python.
- NEVER add metadata JSONB columns to entities, jobs, or context_items. Context is linked via `context-of` relationships.
- NEVER add co-author tags to commits.
- ALWAYS run `make lint` before committing. Pre-commit hooks are mandatory.
- PREFER opus for all work. Use sonnet only for simple CRUD, haiku for formatting.

## Architecture

Monorepo: `cli/` (Go, Bubble Tea TUI), `server/` (Python, FastAPI + MCP), `database/` (Postgres 16 + pgvector).

- **Auth**: Unified API keys for entities and agents (Bearer `nbl_...` tokens)
- **Privacy**: Scope-based access control on every record (`privacy_scope_ids` array intersection)
- **Governance**: Untrusted agent writes go through approval queue, human reviews in CLI
- **Data model**: 19-table scoped knowledge graph. See `@docs/SCHEMA.md` for current ER diagram
- **Schema truth**: SQLAlchemy models in `server/src/nebula_models/`. Alembic for migrations

## Stack Decisions (Locked)

- **SQLAlchemy** for schema definition only, **asyncpg** + **QueryLoader** for runtime queries
- **Pydantic** for input validation, **ruff** for linting (config in `server/ruff.toml`)
- **Bubble Tea** + **Lip Gloss** for TUI, **Cobra** for CLI commands
- **Docker Compose** for postgres only, server and CLI run natively
- **uv** for Python packages, port **6432** locally, creds **nebula/nebula/nebula**

## Commands

```
make help              # all commands
make setup             # fresh clone (deps + db + migrate)
make dev               # API server on port 8765
make build             # build CLI binary
make test              # all tests (server + CLI)
make test-server       # pytest (~1500 tests expected)
make test-cli          # go test (all packages green expected)
make lint              # ruff check + go vet
make format            # ruff format + gofmt
make docs-schema       # regenerate schema docs from models
make db-reset          # full database reset
make install           # install deps + git hooks
```

## Commit Style

Conventional commits: `type(scope): description`

Types: `feat`, `fix`, `refactor`, `docs`, `infra`, `test`, `chore`

## Implementation Pitfalls

- Job IDs are `YYYYQ#-XXXX` text (not UUIDs). Use `_require_job_id()` not `_require_uuid()` for job validation.
- `privacy_scope_ids` is a UUID array, not text. Scope filtering uses `&& $scope_filter` array overlap.
- Approval execution can fail silently if executor is missing. Check `EXECUTORS` dict in `executors.py`.
- Entity/context/job creates pass through `maybe_require_approval()` for untrusted agents. Direct writes only for trusted.
- The `filter_context_segments` function in helpers.py filters relationship properties, not entity metadata. Name is legacy.
- `grep -oP` is not available on ubuntu CI runners. Use `sed`/`awk` for portable parsing.

## Compact Instructions

When compacting this conversation, ALWAYS preserve: the critical rules above, current branch state, test commands, and all prohibitions.

## Do NOT

- Reference vault paths, obsidian, or personal files in repo code
- Delete vault files without explicit permission
- Write assertion-free tests (every test MUST assert something meaningful)
- Skip pre-commit hooks with --no-verify
