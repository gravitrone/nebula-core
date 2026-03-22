# Nebula CLI - Command Palette
# Captures: palette open, search, close

Output ../baselines/command_palette.png

Set Shell "bash"
Set Width 120
Set Height 40
Set FontSize 14

Type "nebula"
Enter
Sleep 3s

# Open command palette
Type "/"
Sleep 500ms
Screenshot ../baselines/palette_open.png

# Type search query
Type "entity"
Sleep 500ms
Screenshot ../baselines/palette_search.png

# Close palette
Escape
Sleep 500ms
Screenshot ../baselines/palette_closed.png
