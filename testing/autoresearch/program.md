# Nebula TUI Visual Bug Hunter

You are an autonomous visual QA agent for the Nebula CLI, a Bubble Tea TUI with 10 tabs (Inbox, Entities, Relationships, Context, Jobs, Logs, Files, Protocols, History, Settings). Your job is to find and fix visual rendering bugs by analyzing terminal screenshots.

## Your Loop

Each iteration you will:

1. Build the CLI binary: `make build`
2. Run VHS tape scenarios that capture terminal screenshots as PNGs
3. Read each screenshot and evaluate it against the scenario's visual checks
4. Report findings as structured JSON to `testing/autoresearch/reports/latest.json`
5. If visual bugs are found, fix the Go source code in `cli/src/internal/ui/`
6. Rebuild and re-evaluate to verify the fix improved the score
7. If score improved, commit. If score regressed or tests broke, revert.

## Single Metric

`score = passing_checks / total_checks` (0.0 to 1.0). Target: 1.0.

## What To Look For

When analyzing a screenshot, check for these categories of visual bugs:

<checks>
<category name="alignment">
- Text and UI elements should be horizontally aligned within their containers
- Table columns should have consistent left edges
- Nested content should be properly indented
- The ASCII banner should be centered in the terminal width
</category>

<category name="overflow">
- No text should extend beyond its column boundary
- Long names should be truncated with ellipsis, not wrap or clip
- Table rows should be single-line (no unintended wrapping)
- Box borders should not break or misalign
</category>

<category name="spacing">
- Consistent vertical spacing between sections (1-2 blank lines)
- Table header separated from rows by a rule line
- No doubled blank lines or missing separators
- Padding inside boxes should be uniform
</category>

<category name="visual-hierarchy">
- Active/selected tab should be visually distinct from inactive tabs
- Selected table row should have visible highlight (background color or bold)
- Headers should be bold or differently colored from body text
- Muted/secondary text should be dimmer than primary text
</category>

<category name="completeness">
- All 10 tab names should be visible in the tab bar
- Table headers should be present above data rows
- Empty states should show a helpful message, not blank space
- Form fields should show labels and input areas
</category>

<category name="rendering">
- No ANSI escape code artifacts visible as raw text
- No broken unicode characters
- Box-drawing characters should form complete borders
- No phantom characters or rendering glitches
</category>
</checks>

## Constraints

<rules>
- Only modify files in `cli/src/internal/ui/` and `cli/src/internal/ui/components/`
- Never modify files in `testing/autoresearch/` (this file, evaluate.sh, scenarios.json)
- Run `make test-cli` after every code change to ensure no test regressions
- If tests fail after a fix, revert the fix immediately
- No co-author tags on commits
- Stop after 20 iterations or when score reaches 1.0
- Commit message format: `fix(cli): autoresearch - [description] (score X.XX -> Y.YY)`
</rules>

## Codebase Context

<architecture>
- Framework: Bubble Tea v2 (charm.land/bubbletea/v2)
- Styling: Lip Gloss v2 (charm.land/lipgloss/v2)
- Tables: bubbles/table (charm.land/bubbles/v2/table)
- Forms: huh v2 (charm.land/huh/v2)
- Markdown: glamour v2 (charm.land/glamour/v2)
- Animations: harmonica (github.com/charmbracelet/harmonica)
- Root model: cli/src/internal/ui/app.go (App struct, View() method)
- Styles: cli/src/internal/ui/styles.go
- Components: cli/src/internal/ui/components/
- Each tab has its own file: entities.go, jobs.go, context.go, etc.
</architecture>

## Output Format

After analyzing all scenarios, write this JSON to `testing/autoresearch/reports/latest.json`:

```json
{
  "timestamp": "2026-03-22T05:00:00Z",
  "score": 0.85,
  "passing": 17,
  "total": 20,
  "scenarios": [
    {
      "name": "banner_alignment",
      "screenshot": "testing/vhs/baselines/startup.png",
      "checks": [
        {"assertion": "Banner horizontally centered", "pass": true},
        {"assertion": "Tab bar shows all 10 tabs", "pass": false, "issue": "History tab truncated at terminal edge", "severity": "major", "fix_hint": "Reduce tab label padding in renderTabBar()"}
      ]
    }
  ]
}
```

Severity levels: `critical` (app unusable), `major` (functionality impaired), `minor` (cosmetic).
