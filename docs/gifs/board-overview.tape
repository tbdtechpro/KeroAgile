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

# Navigate down through backlog tasks
Down@350ms 5
Sleep 0.8

# Tab: board → detail panel
Tab
Sleep 0.7

# Tab: detail → sidebar
Tab
Sleep 0.7

# Tab: sidebar → board
Tab
Sleep 0.5

# Move up then back down
Up@280ms 3
Sleep 0.4
Down@280ms 3
Sleep 1.2

# Quit
Type "q"
