# Nebula CLI - Entity CRUD Flow
# Captures: entity list, add form, detail view

Output ../baselines/entity_crud.png

Set Shell "bash"
Set Width 120
Set Height 40
Set FontSize 14

Type "nebula"
Enter
Sleep 3s

# Switch to entities
Type "2"
Sleep 1s
Screenshot ../baselines/entity_list.png

# Enter content (down from tab nav -> mode focus -> down to table)
Down
Sleep 200ms
Down
Sleep 500ms
Screenshot ../baselines/entity_table_focused.png

# Enter detail view
Enter
Sleep 500ms
Screenshot ../baselines/entity_detail.png

# Back to list
Escape
Sleep 500ms

# Switch to Add mode
Up
Sleep 200ms
Left
Sleep 500ms
Screenshot ../baselines/entity_add_form.png
