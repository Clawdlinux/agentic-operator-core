# Booth Replay Artifacts

These files provide the offline fallback for the Agentic Summit booth.

## Artifacts

- `demo-claude-anf.log` is stdout from a successful live run on July 18, 2026.
- `clawdlinux-claude-anf.gif` is a 41-second browser recording of that replay.
- The GIF is 1152 by 720 pixels and loops continuously.

The live run used `--present --tamper-audit --pace 6` against `kind-clawdlinux-demo`.
It routed the rendered AgentWorkload through LiteLLM to Claude.
No credential values are present in either artifact.

## Replay

Run this from the repository root:

```bash
DEMO_REPLAY_DELAY_SECONDS=0.5 \
  python3 scripts/demo-visualizer.py --replay demos/demo-claude-anf.log
```

Open `http://127.0.0.1:8765` during the countdown.
The dashboard must say `RECORDED REHEARSAL`.
Never present this replay as a live cluster run.

## Evidence Boundaries

- `LIVE` events were live when the source log was recorded.
- `CONFIG ONLY` proves mutation and object presence, not runtime enforcement.
- `PRIOR RUN` verifies the checked-in HMAC fixture and rejects a modified copy.
- The current AgentWorkload did not generate the prior-run audit fixture.

Verify artifact integrity locally with:

```bash
shasum -a 256 demos/demo-claude-anf.log demos/clawdlinux-claude-anf.gif
```
