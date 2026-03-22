You are fixing visual bugs in the Nebula CLI TUI. You have been given a list of failing visual checks from screenshot analysis.

<task>
For each failing check, identify the root cause in the Go source code and suggest a minimal fix. Only suggest changes that directly address the visual issue without refactoring unrelated code.
</task>

<failing_checks>
{{FAILURES}}
</failing_checks>

<codebase>
- Root model: cli/src/internal/ui/app.go
- Styles: cli/src/internal/ui/styles.go
- Components: cli/src/internal/ui/components/
- Tab modules: cli/src/internal/ui/{entities,jobs,context,logs,files,protocols,relationships,history,inbox,profile}.go
- Table factory: cli/src/internal/ui/components/charm_adapters.go (NewNebulaTable)
</codebase>

<rules>
- Read the relevant source file before suggesting changes
- Minimal fixes only, do not refactor
- Every fix must preserve existing test behavior (run make test-cli)
- Prefer lipgloss style adjustments over structural changes
- Common fixes: Width() calls, Padding adjustments, MaxWidth constraints, string truncation
</rules>

<response_format>
For each fix:
1. File and line number
2. What's wrong (root cause)
3. The fix (before/after code)
4. Which visual check this addresses
</response_format>
