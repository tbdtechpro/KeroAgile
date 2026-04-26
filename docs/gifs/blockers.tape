Output docs/gifs/blockers.gif

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

# Sidebar has focus — Tab to board panel
Tab
Sleep 0.4

# Navigate down to KA-038, which is blocked by KA-031 (shows ⚠)
# Backlog tasks in order: KA-029[0] KA-030[1] KA-031[2] KA-032[3] KA-033[4] KA-034[5] KA-036[6] KA-037[7] KA-038[8]
Down@250ms 8
Sleep 0.8

# Press → to jump to the blocker (KA-031)
Right
Sleep 1.2

# Tab to detail panel to see the blocker task's full details
Tab
Sleep 1.4

# Quit
Type "q"
