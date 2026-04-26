Output docs/gifs/board-overview.gif

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

# Sidebar has focus by default — Tab to board panel
Tab
Sleep 0.5

# Navigate down through backlog tasks
Down@300ms 4
Sleep 0.8

# Tab: board → detail panel
Tab
Sleep 0.8

# Tab: detail → sidebar
Tab
Sleep 0.6

# Tab: sidebar → board
Tab
Sleep 0.4

# Toggle Done section open
Type "z"
Sleep 1.6

# Collapse Done section again
Type "z"
Sleep 0.8

# Quit
Type "q"
