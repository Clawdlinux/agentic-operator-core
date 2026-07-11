# booth-loop.tape
# VHS script for the Agentic Summit booth screen loop.
# Install: brew install vhs   (needs ffmpeg + ttyd)
# Record:  vhs video/booth-loop.tape
# Outputs GIF for laptop screens and MP4 for a TV.
#
# Design rules baked in:
# - 55 seconds, no audio, on-screen text only (booth noise kills narration)
# - no hard start or end: someone glancing mid-loop still gets one full beat
# - one beat every ~15-20s: policy deny -> signed chain -> tamper caught
# - brand colors: navy background, ice-blue text, green/red for pass/fail
#
# PREREQ: run against the live demo cluster (scripts/demo-booth.sh --profile lean done once)
# from the repository root. The signed fallback artifact is checked in under _staging/booth/.

Output video/booth-loop.gif
Output video/booth-loop.mp4

Set FontSize 22
Set FontFamily "JetBrains Mono"
Set Width 1920
Set Height 1080
Set Padding 40
Set Theme { "background": "#141B3D", "foreground": "#CADCFC", "green": "#4ADE80", "red": "#F87171", "blue": "#5B8DEF", "cursor": "#5B8DEF" }
Set TypingSpeed 40ms

# ---- Beat 0: standing banner (2s) ----
Type "clear && echo '  Clawdlinux :: can you prove what your agent did?'"
Enter
Sleep 2s

# ---- Beat 1: policy gate (18s) ----
Type "# an agent asks to delete production volumes"
Enter
Sleep 1s
Type "kubectl apply -f config/samples/agentworkload_demo_deny.yaml"
Enter
Sleep 3s
Type "kubectl -n agentic-system get agentworkload opa-deny-demo -o jsonpath='{.status.phase}'"
Enter
Sleep 2.5s
# expected output: PolicyDenied
Type "# DENIED by policy before anything ran. The model never got a vote."
Enter
Sleep 4s

# ---- Beat 2: signed evidence (15s) ----
Type "clear && echo '  every action -> hash-chained, HMAC-signed audit entry'"
Enter
Sleep 1s
Type "audit-verify --source jsonl --path _staging/booth/attestation-fallback.jsonl --key booth-demo-2026=bmluZXZpZ2lsLWJvb3RoLWRlbW8tYXR0ZXN0YXRpb24ta2V5LTMyYg=="
Enter
Sleep 3s
# expected output: chain OK ... PASS
Type "# PASS. An auditor can recompute this offline. No trust required."
Enter
Sleep 4s

# ---- Beat 3: tamper caught (15s) ----
Type "# now someone edits one row of history..."
Enter
Sleep 1s
Type "sed 's/\"actor\":\"policy-analyst\"/\"actor\":\"ghost-actor\"/' _staging/booth/attestation-fallback.jsonl > /tmp/tampered.jsonl"
Enter
Sleep 2s
Type "audit-verify --source jsonl --path /tmp/tampered.jsonl --key booth-demo-2026=bmluZXZpZ2lsLWJvb3RoLWRlbW8tYXR0ZXN0YXRpb24ta2V5LTMyYg=="
Enter
Sleep 3s
# expected output: FAIL at seq=4 ... entry_hash mismatch
Type "# CAUGHT. Signed, not just logged."
Enter
Sleep 3s

# ---- Loop point: QR CTA (5s) ----
Type "clear && echo '  github.com/Clawdlinux  |  free pilot for regulated teams  |  scan the QR'"
Enter
Sleep 4s
