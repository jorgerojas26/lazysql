---
# lazysql-zca0
title: '[03.1] Use Foreign Key Jump vocabulary consistently'
status: completed
type: bug
priority: normal
tags:
    - ready-for-agent
    - order-03-01
    - review-finding
    - review-cycle-0002
created_at: 2026-09-12T20:42:44Z
updated_at: 2026-09-12T21:04:00Z
parent: lazysql-a9lf
---

## Review source

Review of `v0.5.7...HEAD` (fixed point `a07385a07523428234e350727f9e769f464e302e`, pinned HEAD `8bdaefccbb191f33a5285f7f25bec1a41b1b1c03`).

Owning Bean: `lazysql-846w — [03] Enrich Records progressively with cached table metadata`

Axes:
- Standards: The initiative and Bean 03 use “FK-jump” and “composite-FK navigation” instead of the glossary's exact **Foreign Key Jump** term.
- Spec: none

## Observed behavior

The reviewed parent initiative and Bean 03 use alternate names for the defined relation-navigation concept. `docs/agents/domain.md` requires issue/spec language to use the glossary vocabulary, while `CONTEXT.md` defines **Foreign Key Jump** and explicitly rejects synonym drift. This makes the same capability appear under multiple names in its owning specification and acceptance criteria.

## What to build

Normalize the reviewed initiative and Bean documentation to the exact **Foreign Key Jump** domain term while preserving the intended supported-provider and composite-key non-goals. Prevent the same rejected abbreviations or synonyms from returning in tracked specification text.

## Acceptance criteria

- [x] Replace “FK-jump” and alternate relation-navigation names in the reviewed initiative/Bean sources with the exact **Foreign Key Jump** term.
- [x] Preserve the substantive meaning of supported-provider behavior and the composite-key navigation non-goal.
- [x] Search the reviewed specification and Bean sources for remaining rejected synonyms and correct any references to this domain concept.
- [x] Add a lightweight vocabulary check or equivalent regression guard that fails for the reviewed drift and passes after correction.
- [x] Preserve `lazysql-846w`'s functional Foreign Key Jump acceptance criteria unchanged apart from terminology.

## Blocked by

None. No implementation prerequisite is required.

## Summary of Changes

- Replaced abbreviated and composite-key relation-navigation wording with the exact Foreign Key Jump glossary term in the initiative and Bean 03 specification.
- Added `internal/vocabulary_test.go` to guard the reviewed specification sources against rejected Foreign Key Jump synonyms.
- Preserved the supported-provider and composite-key non-goals and Bean 03 functional acceptance behavior.
