# Naming Decision — ACP protocol + Agentic Operator

Date: 2026-06-16. Status: RESEARCH COMPLETE. Needs human approval (gate A2).
Author: ralph loop, Lane A1.

## TL;DR recommendation

- Protocol: STOP using the bare acronym "ACP". It is triple-claimed and one
  claimant sits in our exact space. Pick a unique product mark and keep
  "Agent Contract Protocol" only as a descriptive subtitle.
  Recommended mark: `Concord` (full: "Concord: an Agent Contract Protocol").
  Runner-up: `Tacit`. Both pending a 10-minute availability check at rename time.
- Operator: keep the public repo name `agentic-operator-core`. It already has
  history and a star. Do NOT rebrand the repo. DO move the Go module path off
  the personal handle. Use `github.com/clawdlinux/agentic-operator`.

## Human decision needed (gate A2)
1. Approve a protocol mark: `Concord`, `Tacit`, or your own. One word back.
2. Approve the operator module path: `github.com/clawdlinux/agentic-operator`.
3. Confirm we keep "Agent Contract Protocol" as the descriptive subtitle (the
   contract framing matches declared intent + ordering + auth-out-of-context).

## Why "ACP" is dead (evidence, verified 2026-06-16)

The acronym collides at least three ways. Two are confirmed by direct fetch:

- agent-context-protocol — live GitHub org. Repo self-describes as "The first
  protocol for multi-agent communication and coordination." 8 stars, MIT,
  active 2025. https://github.com/agent-context-protocol
- Agent Client Protocol (Zed) — brands itself "ACP" on the landing page,
  standardizes editor<->coding-agent comms, JSON-RPC over stdio, reuses MCP
  JSON. This is our exact adjacent space and our exact transport story (stdio +
  MCP reuse). https://agentclientprotocol.com  repo:
  https://github.com/agentclientprotocol/agent-client-protocol
- Agent Communication Protocol (IBM / Linux Foundation, BeeAI lineage) — widely
  cited "ACP". Known prior art; not re-fetched this run, verify the URL before
  citing it in the paper.

Conclusion: the acronym is unwinnable. Zed's ACP alone is fatal for
differentiation because it shares our transport and MCP-reuse story. Nobody can
search, cite, or star a fourth "ACP". The arXiv paper title would land in
contested namespace on day one.

## Candidate scoring (protocol)

Availability columns marked CHECK could not be auto-verified: pypi.org returns a
browser-challenge to the fetcher. Verify with `pip index versions <name>` or a
browser at rename time before committing the name.

| Candidate | Acronym risk | GitHub org | PyPI name | Paper-title unique | Describes wedge | Verdict |
|---|---|---|---|---|---|---|
| Keep "Agent Contract Protocol" (ACP) | FAIL — triple-claimed, Zed in-space | n/a | n/a | FAIL | yes | reject the acronym, keep as subtitle only |
| Concord (concord-protocol) | low — common word, not an agent-protocol | CHECK | CHECK (concord-protocol / pyconcord) | likely unique as "Concord agent contract protocol" | medium — "agreement/contract between agent and tools" | RECOMMEND |
| Tacit (tacit-protocol) | low | CHECK | CHECK | likely unique | medium — "tacit contract" / implied intent | runner-up |
| Intent Resolution Protocol (IRP) | medium — IRP = I/O Request Packet, routing | CHECK | CHECK | medium | high — names the mechanism | viable but acronym noise |

Rationale for `Concord`: it is a real word meaning agreement/contract, which
maps onto the product (a declared contract between an agent and its tools). It
gives a clean, ownable namespace (concord-protocol, pyconcord) away from the ACP
acronym wars, while "Agent Contract Protocol" survives as the honest one-line
descriptor. `Tacit` is the runner-up: shorter, evocative of implied/declared
intent, but less self-explanatory.

## Operator

- Public name: keep `agentic-operator-core`. Changing the slug throws away the
  little history and the one star, and the name is fine. The README is the real
  problem, not the name (handled in Lane B3).
- Go module path: today it is `github.com/Clawdlinux/agentic-operator-core`, a
  personal handle. This reads unserious and ties the import path to one person.
  Move it to `github.com/clawdlinux/agentic-operator` to match the public org.
  This is a v0.x project, so a module-path change is acceptable now and only
  gets more expensive later.
- This rename touches go.mod, every internal import, and any `go install` URL in
  docs. It is the A4 human gate. Stage it as a script, do not run it unattended.

## Next steps
- A2 (human): pick the mark + confirm module path.
- A3 (loop, local): mechanical rename of README/paper/docs/package metadata to
  the approved mark. Verify PyPI + GitHub-org availability first.
- A4 (human): repo-slug decision (operator: keep) + run the staged
  module-path rename script.
