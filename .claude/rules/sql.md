---
paths: ["server/src/queries/**/*.sql"]
---

# SQL Conventions

- Every file starts with `-- <short description>` comment
- Use `$1, $2, ...` parameterized placeholders (asyncpg style)
- RETURNING * on INSERT/UPDATE queries
- Use explicit column lists on INSERT (never INSERT INTO table VALUES)
- UUID columns use `gen_random_uuid()` server default
- Timestamps use `now()` server default with timezone
- Privacy filtering: `privacy_scope_ids && $scope_filter` for array overlap
