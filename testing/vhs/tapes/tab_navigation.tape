# Nebula CLI - Tab Navigation
# Captures: all 10 tabs via number keys

Output ../baselines/tab_navigation.png

Set Shell "bash"
Set Width 120
Set Height 40
Set FontSize 14

Type "nebula"
Enter
Sleep 3s

# Cycle through all tabs
Type "2"
Sleep 500ms
Screenshot ../baselines/tab_entities.png

Type "3"
Sleep 500ms
Screenshot ../baselines/tab_relationships.png

Type "4"
Sleep 500ms
Screenshot ../baselines/tab_context.png

Type "5"
Sleep 500ms
Screenshot ../baselines/tab_jobs.png

Type "6"
Sleep 500ms
Screenshot ../baselines/tab_logs.png

Type "7"
Sleep 500ms
Screenshot ../baselines/tab_files.png

Type "8"
Sleep 500ms
Screenshot ../baselines/tab_protocols.png

Type "9"
Sleep 500ms
Screenshot ../baselines/tab_history.png

Type "0"
Sleep 500ms
Screenshot ../baselines/tab_settings.png

Type "1"
Sleep 500ms
Screenshot ../baselines/tab_inbox.png
