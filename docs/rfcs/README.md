# RFCs (Request for Comments)

This directory holds design proposals for significant architectural changes to NineVigil. RFCs are public design artifacts opened for community input *before* implementation begins.

## Lifecycle

1. **Draft** — authored as `NNNN-short-title.md`, paired with a GitHub Discussion in the `Ideas` category
2. **Open for Comment** — community feedback collected in the discussion
3. **Accepted** — meets validation gate (typically N+ external use cases or paying customer demand), tracked as an epic in Issues
4. **Implemented** — merged via a milestone; RFC becomes historical reference
5. **Rejected / Withdrawn** — kept in repo for institutional memory

## Index

| ID | Title | Status | Discussion |
|----|-------|--------|------------|
| [0001](0001-cross-cluster-agent-identity.md) | Cross-Cluster Agent Identity Federation via SPIFFE/SPIRE | Draft — Open for Comment | _(pending)_ |

## Authoring a new RFC

1. Copy the structure of [0001](0001-cross-cluster-agent-identity.md)
2. Use the next sequential number, four digits, zero-padded
3. Include: Abstract, Motivation, Background, Goals/Non-Goals, Design, Security, Migration, Open Questions, Alternatives, Validation Gate, References, Decision Log
4. Keep total length under 8 pages — if longer, split into multiple RFCs
5. Open a GitHub Discussion in the `Ideas` category and link it in the RFC frontmatter
6. Submit as a PR to this directory
