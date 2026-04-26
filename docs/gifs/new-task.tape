Output docs/gifs/new-task.gif

Set FontSize 15
Set Width 1600
Set Height 860
Set Framerate 30
Set TypingSpeed 80ms
Set Theme "Catppuccin Mocha"
Set WindowBar Colorful

Hide
Type "cd /home/matt/github/KeroAgile"
Enter
Type "clear"
Enter
Show

Type "./KeroAgile"
Enter
Sleep 1.8

# Open new task form
Type "n"
Sleep 0.6

# Type task title
Type "Add keyboard shortcut help overlay"
Sleep 0.5

# Tab to description — skip it
Tab
Sleep 0.3

# Tab to assignee — leave as auto-suggested
Tab
Sleep 0.4

# Tab to priority — change to high
Tab
Sleep 0.3
Ctrl+A
Type "high"
Sleep 0.4

# Tab to points
Tab
Sleep 0.3
Type "3"
Sleep 0.4

# Tab through status and labels
Tab
Sleep 0.2
Tab
Sleep 0.2

# Tab to sprint — save from here (Enter on a non-blocker field saves the form)
Tab
Sleep 0.3

# Save
Enter
Sleep 1.8

# Quit
Type "q"
