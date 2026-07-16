# Claude ANF Showcase Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Build a paced, visually legible showcase that translates live Kubernetes state into ANF, sends that context through an AgentWorkload to Claude, and displays each proof boundary in real time.

**Architecture:** A small Go command snapshots one namespace with client-go and records source payload accounting. It translates projected objects through the public Agent Native Format library pinned to commit `651eea0`. The command writes the ANF document and compares its exact JSON and ANF encodings. `demo-booth.sh` injects that ANF document into a temporary AgentWorkload manifest, routes every task category to the Anthropic LiteLLM alias, and pauses between evidence stages. The existing Python SSE visualizer parses ANF, Claude, controls, and audit events into one branded control room.

**Tech Stack:** Go 1.25, client-go, Agent Native Format Go library, Bash, Python standard library, HTML/CSS/JavaScript, LiteLLM, Claude Haiku 4.5.

---

## Proof Contract

- `LIVE`: Kubernetes snapshot, ANF translation metrics, AgentWorkload completion, Claude tokens, estimated cost.
- `CONFIG ONLY`: gVisor admission dry-run and NetworkPolicy object presence. No pod or packet-test claim.
- `PRIOR RUN`: HMAC audit fixture verification and tampered-copy rejection.
- Source bytes and omission counts account for the observed Kubernetes payload.
- ANF reduction compares JSON and ANF encodings of the exact same `anf.Document`.
- Token counts use a chars-per-token estimate and are labeled as estimates.
- The ANF document is appended to the actual AgentWorkload objective sent to Claude.
- The showcase path is Claude-only. Product-level OpenAI compatibility remains supported but is not rendered, narrated, or required by the showcase.
- Every stage pause is configurable. Tests and CI use zero delay.

### Task 1: Live ANF Namespace Snapshot

**Files:**
- Create: `cmd/anf-snapshot/main.go`
- Create: `cmd/anf-snapshot/main_test.go`
- Create: `internal/anfsnapshot/snapshot.go`
- Create: `internal/anfsnapshot/snapshot_test.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `Makefile`

**Requirements:**
- Pin `github.com/Clawdlinux/agent-native-format` to pseudo-version for commit `651eea0fc411d34d94807fd233968e6c0c93ab9f` because existing tags have the pre-rename module path.
- Fetch Deployments, Pods, Services, Jobs, and CronJobs from one namespace using context-aware client-go calls.
- Convert observed objects to `translators/kubernetes.NamespaceView` without inventing CPU, memory, traffic, or error-rate data.
- Marshal fetched lists only for source payload bytes, object counts, and omission accounting.
- Count unprojected Pods, extra deployment containers, extra service ports, and named first target ports.
- Marshal the exact translated `anf.Document` as JSON.
- Encode that same document with `pkg/anf.EncodeToString`.
- Write ANF to `--output` with mode `0600`.
- Print one parseable summary line:
  `ANF context: source=... scope=... source_bytes=N ... document_json_bytes=N anf_bytes=N document_json_tokens_est=N anf_tokens_est=N reduction=P ...`
- Print at most 3 `ANF preview:` lines. Never dump the full raw JSON.
- Fail closed on any required list error. No partial snapshot may be labeled live.
- Add `make build-anf-snapshot`.

**Validation:**
```bash
go test ./internal/anfsnapshot ./cmd/anf-snapshot
go test -race ./internal/anfsnapshot ./cmd/anf-snapshot
make build-anf-snapshot
```

**Commit:** `feat(demo): add live ANF namespace snapshot`

### Task 2: Claude-Only Paced Showcase Path

**Files:**
- Modify: `examples/research-agent.yaml`
- Modify: `scripts/demo-booth.sh`
- Modify: `tests/smoke/test_demo_booth_cli.sh`
- Modify: `charts/charts/litellm/templates/configmap.yaml`
- Modify: `charts/charts/litellm/values.yaml`
- Modify: `charts/tests/litellm_existing_secret_test.yaml`
- Modify: `charts/values.schema.json`
- Modify: `pkg/finops/memory_reporter.go`
- Modify: `pkg/finops/memory_reporter_test.go`

**Requirements:**
- Map validation, analysis, and reasoning to `litellm/clawdlinux-anthropic`.
- Label the route as Claude Haiku 4.5 through in-cluster LiteLLM.
- Add correct explicit pricing for the Anthropic alias, sourced from current official pricing before implementation.
- Make the booth chart disable built-in OpenAI models while retaining product defaults for other installs.
- Require Anthropic credentials for the showcase. OpenAI credentials may remain optional for non-showcase compatibility but may not appear in showcase output.
- Build or locate `anf-snapshot`, write a temporary `0600` ANF file, and inject it into a temporary JSON AgentWorkload manifest using `kubectl --dry-run=client -o json` plus Python's JSON library.
- Apply the temporary manifest with repo-local `agentctl` when available.
- Remove the separate Anthropic reachability request. The hero AgentWorkload itself is the Claude proof.
- Add `--pace SECONDS` and `DEMO_STAGE_DELAY_SECONDS`. Default `6`; tests use `0`. Allow only non-negative integers.
- Pause after ANF translation, Claude evidence, controls evidence, and first audit verification. Print `Narration pause: Ns` before each delay.
- Preserve the 90-second live fallback policy outside the script.
- No secret values in output or command arguments.

**Validation:**
```bash
bash -n scripts/demo-booth.sh
bash tests/smoke/test_demo_booth_cli.sh
helm unittest charts
go test ./pkg/finops
```

**Commit:** `feat(demo): route paced showcase through Claude`

### Task 3: Visualize ANF and Claude Proof

**Files:**
- Modify: `scripts/demo-visualizer.py`
- Modify: `scripts/demo-dashboard.html`
- Modify: `scripts/demo_visualizer_test.py`

**Requirements:**
- Parse the ANF summary and preview lines as `LIVE` evidence.
- Render provider path as `Kubernetes → ANF context → AgentWorkload → LiteLLM → Claude Haiku 4.5`.
- Add a compact band with source bytes, omission counts, document JSON bytes, ANF bytes, estimated reduction, and entity count.
- Keep the 3 existing evidence panels for model, controls, and prior-run audit.
- Remove all OpenAI-specific display logic and replay fixtures from tests.
- Use event pacing without browser animation that changes the underlying proof.
- Preserve replay labeling, SSE reliability, projector breakpoints, and evidence scopes.

**Validation:**
```bash
python3 -m unittest scripts.demo_visualizer_test
python3 -m py_compile scripts/demo-visualizer.py scripts/demo_visualizer_test.py
```

**Commit:** `feat(demo): visualize live ANF context for Claude`

### Task 4: Rehearsal Artifact and Walkthrough

**Files:**
- Modify: `docs/SHOWCASE-DEMO-WALKTHROUGH.md`
- Modify: `docs/PITCH-SCRIPT.md`
- Add: `demos/demo-claude-anf.log` only if repository policy allows recorded artifacts.

**Requirements:**
- Replace screening-specific timing with a 7-minute showcase flow and a 3-minute screening variant.
- Include the problem, market signal, Clawdlinux solution, ANF role, live proof, evidence limits, ask, and expected questions.
- Remove showcase OpenAI references.
- Explain that ANF is a token-minimal live-system view built with Claude, but do not claim Claude generated every line or that ANF alone improves model accuracy unless measured.
- Keep the 90-second live fallback. Replay must remain visibly labeled.
- Run a full live rehearsal with `--pace 0` for validation, then a paced rehearsal with `--pace 6` for timing.
- Record a new Claude + ANF fallback. Do not use the old OpenAI rehearsal for the showcase.

**Validation:**
```bash
./scripts/demo-booth.sh --prepare
./scripts/demo-booth.sh --present --tamper-audit --pace 0
python3 scripts/demo-visualizer.py --replay demos/demo-claude-anf.log
```

**Commit:** `docs(demo): add Claude ANF showcase walkthrough`

### Task 5: Full Review and Delivery

- Run focused and full tests, Helm lint, Go build/vet, Python syntax, secret scan, and browser checks.
- Capture control-room screenshots at 1920x1080, 1440x900, and 1280x800.
- Verify no clipped content or horizontal overflow.
- Search showcase artifacts for `OpenAI`, `clawdlinux-openai`, and stale replay references. Product compatibility code is excluded from this prohibition.
- Dispatch final spec and code-quality reviews.
- Open a signed-off PR, wait for CI, squash-merge, and delete branches.

## Explicit Non-Goals

- No chat interface or OpenCode plugin.
- No claim that gVisor executed on kind/macOS.
- No claim that NetworkPolicy packet enforcement was tested.
- No claim that the prior-run audit fixture came from the current Claude workload.
- No claim that ANF is already integrated into every production reconciliation.
- No removal of provider-agnostic OpenAI support from the product.