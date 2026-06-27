# Ralph Backlog — agentic-operator-core

Ordered task queue for the review-execution ralph loop. One task = one signed-off
commit on a feature branch. States: `[ ]` todo, `[~]` in progress, `[x]` done,
`[HUMAN]` needs the human gate (loop stops and flags).

Pre-flight gates before any new branch or issue (do not skip):
- `gh pr list --state open` before branching. Build on an existing branch if scope overlaps.
- `gh issue list --state open --search "<keywords>"` before filing any issue.
- Feature branch + PR, squash merge, delete branch. `git commit -s`. No coauthor. No emojis.

## Lane B — public readiness
- [x] B1 Commit in-flight runtime-agnostic work on feat/runtime-agnostic-adapter (README, ROADMAP, charts, RFC-0001, adapter.go+_test, strategist sample) + untracked (CLAUDE.md, STRATEGY doc, agentworkload.yaml, tenants CRD). Verify `go build ./...` + `go test ./pkg/runtime/...` first.
- [ ] B2 Review adapter interface for 3-runtime parity (BYO pod / AgentWorkload / kagent Agent), identical egress-seal + attestation. Confirm v1alpha2 runtimeClassName note current. skills: code-reviewer + architect-review.
- [ ] B3 README rewrite: lead with zero-egress seal + tamper-evident in-cluster attestation artifact. Move "For VCs & Reviewers" below the fold. Add `helm install` one-liner. Keep runtime-agnostic framing; never make kagent the subject of a problem sentence. (name from A3 when ready). skill: dx-optimizer.
- [x] B4 Hygiene: already clean as of 2026-06-16. .serena/ untracked, .gitignore covers .keys/ .DS_Store .serena/. No videos/pptx/demo-day at root. (Review hygiene complaint was stale.)
- [HUMAN] B5 Open PR for B1-B4, security-auditor quick pass, squash-merge to main, delete branch.
- [ ] B6 After merge: file/refresh roadmap issues for social proof (search existing first). skill: docs-architect.

## Lane A — naming (blocks B3 title, repo slug, module path)
- [x] A1 Acronym-collision matrix + one recommendation -> naming-decision.md (shared with ACP). Recommend: drop ACP acronym, mark `Concord`, operator module path github.com/clawdlinux/agentic-operator.
- [HUMAN] A2 Approve the name.
- [ ] A3 Mechanical local rename: README, docs, badges, chart metadata. LOCAL only.
- [HUMAN] A4 Irreversible: GitHub repo slug + Go module path (github.com/Clawdlinux/agentic-operator-core). Staged as rename-module.sh.

## Notes
- Go module today: github.com/Clawdlinux/agentic-operator-core (not under Clawdlinux org — naming debt).
- Strategy of record: STRATEGY_positioning_runtime_agnostic.md. Runtime-agnostic, kagent is one adapter.
