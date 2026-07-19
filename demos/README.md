# Local Booth Recordings

Generated recordings are local booth assets. They are not source artifacts and are ignored by Git.

## Record the End-to-End Flow

Run this from the repository root on macOS:

```bash
scripts/record-demo.sh --scenario all --pace 3 --duration 120 --no-open
```

Install the local recording dependencies once:

```bash
npm install -g playwright
playwright install ffmpeg
```

The recorder starts the real visualizer command and records an isolated 1920 by 1080 dashboard as WebM.
It records both editable scenarios through one dashboard connection.
It verifies scenario results, ANF, completion, Claude, cost, audit, and tamper evidence.

Scenario source files live under `examples/booth-scenarios/`.
Edit the fixture and workload YAML before recording when you need another use case.

Generated files live under `demos/local/`:

- `latest.webm` points to the newest end-to-end browser recording.
- `latest.log` points to stdout from the same real run.

## Replay the Recorded Stdout

```bash
DEMO_REPLAY_DELAY_SECONDS=0.5 \
  python3 scripts/demo-visualizer.py --replay demos/local/latest.log
```

Open `http://127.0.0.1:8765` during the countdown.
The dashboard must say `RECORDED REHEARSAL`.
Never present replayed stdout or `latest.webm` as the current live cluster run.

## Evidence Boundaries

- `LIVE` events were live when the local source log was recorded.
- `CONFIG ONLY` proves mutation and object presence, not runtime enforcement.
- `PRIOR RUN` verifies the HMAC fixture and rejects a modified copy without a recomputed MAC.
- The current AgentWorkload did not generate the prior-run audit fixture.
