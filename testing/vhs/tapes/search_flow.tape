# Nebula CLI - Search Flow
# Captures: search tab with results

Output ../baselines/search_flow.png

Set Shell "bash"
Set Width 120
Set Height 40
Set FontSize 14

Type "nebula"
Enter
Sleep 3s

# Open command palette and search
Type "/"
Sleep 500ms
Type "test"
Sleep 1s
Screenshot ../baselines/search_results.png

Escape
Sleep 500ms
