---
paths: ["server/tests/**/*.py", "cli/**/*_test.go"]
---

# Testing Conventions

- Every test MUST assert something meaningful. No assertion-free View() calls.
- NEVER use NotPanics as the sole assertion. Test actual state/output.
- PREFER integration tests hitting real postgres over mocked unit tests.
- AVOID mock-everything tests where assertion is "mock returned mock value".
- PREFER table-driven/parametrized tests over copy-paste test functions.
- Python: use `pytest.raises(ValidationError)` for validation tests.
- Go: use `require` for fatal checks, `assert` for non-fatal.
- Test names describe the scenario: `test_create_entity_rejects_invalid_scope`
