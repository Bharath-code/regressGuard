# vhs tape: break → detect → fix → green, recorded in fixtures/nextjs-app.
# Prereqs: fixture dev server running on :3000 (routes warmed), ./rg built at repo root.
# Record with:  vhs demo/demo.tape   (from the repo root)
Output demo/demo.gif

Set FontSize 15
Set Width 920
Set Height 560
Set TypingSpeed 30ms
Set Padding 12

Hide
Type "cd fixtures/nextjs-app && alias rg=../../rg && clear"
Enter
Sleep 1s
Show

Type "# Record the known-good baseline before the AI session"
Enter
Type "rg snapshot"
Enter
Sleep 12s

Type "# ...an AI agent edits the code and silently drops a field..."
Enter
Hide
Type `sed -i '' '/subscription: "pro",/d' app/api/profile/route.ts`
Enter
Sleep 1s
Type "clear"
Enter
Show

Type "rg check"
Enter
Sleep 10s

Type "# agents get the culprit file as a structured hint:"
Enter
Type "rg check --json | jq -r '.results[0].hint'"
Enter
Sleep 8s

Type "# the agent fixes it and re-checks:"
Enter
Hide
Type "git checkout -- app/api/profile/route.ts && clear"
Enter
Sleep 1s
Show

Type "rg check"
Enter
Sleep 10s
Sleep 3s
