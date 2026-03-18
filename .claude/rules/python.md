---
paths: ["server/**/*.py"]
---

# Python Conventions

- Module docstring on every .py file: `"""Context API routes."""`
- Imports sorted by ruff automatically. No section comments.
- Section separators: `# --- Section Name ---` with proper capitalization
- Google-style docstrings, summary on opening `"""` line, proper capitalization
- ALWAYS leave one blank line after closing `"""`
- Mid-function imports ONLY for circular dependency avoidance
- All SQL in `.sql` files via QueryLoader. NEVER inline SQL.
- Every `.sql` file starts with `-- <short description>`
