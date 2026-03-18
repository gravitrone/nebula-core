---
paths: ["cli/**/*.go"]
---

# Go Conventions

- Standard goimports grouping: stdlib, external, local (blank line separators)
- Section separators: `// --- Section Name ---` with proper capitalization
- Exported functions MUST have doc comments starting with function name
- All API types use `json` struct tags
- Error messages lowercase: `fmt.Errorf("failed to create entity: %w", err)`
- NEVER panic. Return errors with `%w` wrapping.
- Bubble Tea: Model-Update-View pattern. Messages are types, not strings.
