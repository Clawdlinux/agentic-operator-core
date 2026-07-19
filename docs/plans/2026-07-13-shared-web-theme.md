# Shared Web Theme Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Create one reusable Clawdlinux web theme and apply it to both product demo surfaces.

**Architecture:** `pkg/webtheme` owns framework-neutral CSS tokens and canonical SVG assets. Go embeds the package. Python serves the same files. Both dashboards reference those assets over HTTP.

**Tech Stack:** Go `embed.FS`, Python standard library HTTP server, HTML, CSS, HTMX, Server-Sent Events.

---

## Design Contract

- Direction: containment console with editorial security instrumentation.
- Brand colors: night `#05080f`, ice `#e2e8f0`, signal `#60a5fa`, evidence `#00d4aa`.
- Typography: Space Grotesk for display text. IBM Plex Mono for evidence and metrics.
- Logo: canonical sealed-boundary mark and wordmark from the website.
- Radius: no more than `8px`.
- Motion: one entrance sequence and state transitions only.
- Every proof must say `LIVE`, `CONFIG ONLY`, or `PRIOR RUN`.
- No OPA, packet enforcement, scheduled gVisor, or current-run audit claims.

### Task 1: Shared Theme Package

**Files:**
- Create: `pkg/webtheme/embed.go`
- Create: `pkg/webtheme/embed_test.go`
- Create: `pkg/webtheme/assets/clawdlinux-theme.css`
- Create: `pkg/webtheme/assets/clawdlinux-mark.svg`
- Create: `pkg/webtheme/assets/clawdlinux-wordmark.svg`
- Modify: `cmd/agentctl-web/main.go`

**Step 1: Write failing package tests**

Test that `webtheme.FS()` contains all 3 assets. Assert required token names and canonical colors.

**Step 2: Verify failure**

Run: `go test ./pkg/webtheme`

Expected: package or symbols missing.

**Step 3: Add minimal package**

Embed `assets/*`. Return the assets subtree through `FS()`. Add canonical website tokens and SVGs.

Mount the returned filesystem at `GET /theme/` in both `agentctl-web` modes.

**Step 4: Verify package and server**

Run: `go test ./pkg/webtheme ./cmd/agentctl-web`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/webtheme cmd/agentctl-web/main.go
git commit -s -m "feat(ui): add shared Clawdlinux web theme"
```

### Task 2: Live Demo Control Room

**Files:**
- Modify: `scripts/demo-visualizer.py`
- Modify: `scripts/demo-dashboard.html`
- Create: `scripts/demo_visualizer_test.py`

**Step 1: Write failing Python tests**

Test the theme endpoints. Test replay labeling. Test proof categories from parsed events.

**Step 2: Verify failure**

Run: `python3 -m unittest scripts.demo_visualizer_test`

Expected: missing theme endpoints or labels.

**Step 3: Serve the shared package**

Serve CSS and SVG files from `pkg/webtheme/assets`. Keep the Python server dependency-free.

**Step 4: Replace the dashboard layout**

Build one stable 16:9 control room. Show provider path, evidence rail, audit receipt, truth strip, and 5-line event tail.

Use the shared theme CSS and wordmark. Label replay mode as `RECORDED REHEARSAL`.

**Step 5: Verify**

Run:

```bash
python3 -m unittest scripts.demo_visualizer_test
python3 -m py_compile scripts/demo-visualizer.py
python3 scripts/demo-visualizer.py --replay demos/demo-20260713.log
```

Expected: tests pass and replay remains readable.

**Step 6: Commit**

```bash
git add scripts/demo-visualizer.py scripts/demo-dashboard.html scripts/demo_visualizer_test.py
git commit -s -m "feat(demo): add branded live control room"
```

### Task 3: Governance Evidence Console

**Files:**
- Modify: `cmd/agentctl-web/templates/layout.html`
- Modify: `cmd/agentctl-web/templates/demo.html`
- Modify: `cmd/agentctl-web/static/style.css`
- Modify: `cmd/agentctl-web/demo_test.go`

**Step 1: Write failing template tests**

Assert shared CSS and wordmark usage. Reject stale OPA, GPT-4o, and current audit claims.

**Step 2: Verify failure**

Run: `go test ./cmd/agentctl-web`

Expected: stale claim assertions fail.

**Step 3: Apply shared theme**

Load `/theme/clawdlinux-theme.css`. Use canonical wordmark. Style all pages with shared tokens.

Replace static proof content with honest live state, declared evidence boundaries, and review workflow language.

**Step 4: Verify**

Run: `go test ./cmd/agentctl-web ./pkg/webtheme`

Expected: PASS.

**Step 5: Commit**

```bash
git add cmd/agentctl-web
git commit -s -m "feat(ui): align evidence console with Clawdlinux brand"
```

### Task 4: Visual and Integration Validation

**Files:**
- Modify only if validation finds a defect.

**Step 1: Run executable checks**

```bash
go test ./pkg/webtheme ./cmd/agentctl-web
python3 -m unittest scripts.demo_visualizer_test
python3 -m py_compile scripts/demo-visualizer.py
make build-agentctl-web
```

**Step 2: Capture browser evidence**

Check both surfaces at `1440x900`, `1920x1080`, and `1280x800`.

Verify no overlap, horizontal scroll, blank regions, or unreadable projector text.

**Step 3: Verify proof boundaries**

Search rendered text. Confirm no unsupported live OPA, packet, gVisor execution, or audit-generation claims.

**Step 4: Commit validation fixes**

Use a signed commit only when changes were required.

## Explicit Non-Goals

- No React package or JavaScript component library.
- No chat interface.
- No OpenCode plugin.
- No new backend API.
- No current-run audit wiring.
- No changes to the website repository.