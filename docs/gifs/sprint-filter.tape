Output docs/gifs/sprint-filter.gif

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

# Sidebar has focus by default — navigate to KCP project
Down
Sleep 0.5
Enter
Sleep 1.0

# Sidebar now shows KCP's sprint list — navigate to Phase 1C
Down
Sleep 0.4
Down
Sleep 0.4
Enter
Sleep 1.0

# Tab to board to browse filtered tasks
Tab
Sleep 0.5

Down@300ms 4
Sleep 0.6

# Tab to detail to see task info
Tab
Sleep 1.0

# Tab back to sidebar
Tab
Sleep 0.4
Tab
Sleep 0.4

# Back to project list
Escape
Sleep 0.8

# Quit
Type "q"
