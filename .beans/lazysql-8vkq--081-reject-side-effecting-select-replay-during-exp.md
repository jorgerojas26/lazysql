---
# lazysql-8vkq
title: '[08.1] Reject side-effecting SELECT replay during export'
status: todo
type: bug
priority: high
tags:
    - ready-for-agent
    - order-08-01
    - review-finding
    - review-cycle-0002
created_at: 2026-09-12T20:42:44Z
updated_at: 2026-09-12T20:42:44Z
parent: lazysql-a9lf
---

## Review source

Review of `v0.5.7...HEAD` (fixed point `a07385a07523428234e350727f9e769f464e302e`, pinned HEAD `8bdaefccbb191f33a5285f7f25bec1a41b1b1c03`).

Owning Bean: `lazysql-xq1u — [08] Stream CSV exports safely from table and query results`

Axes:
- Standards: none
- Spec: Export All misclassifies a side-effecting MSSQL SELECT as replay-safe.

## Observed behavior

The replay classifier accepts a leading `SELECT` unless tokens match its denylist or unknown function-call rules. MSSQL `SELECT NEXT VALUE FOR dbo.sequence_name` contains no currently denied token or parenthesized function call, so Export All is offered and reexecutes the statement, advancing the sequence a second time.

This contradicts parent spec §27 and `lazysql-xq1u`: unknown or potentially mutating result-producing SQL must not be automatically reexecuted, and false positives that repeat side effects are unacceptable.

## What to build

Make Export All eligibility conservative for result-producing SQL constructs that can mutate server state without function-call syntax. A user who runs such a statement may export the visible rows, but LazySQL must not offer automatic replay. Preserve Export All for statements that can actually be established as replay-safe.

## Acceptance criteria

- [ ] `SELECT NEXT VALUE FOR ...` and equivalent sequence-advancing MSSQL forms are refused for Export All and remain eligible for Export Visible Results.
- [ ] Regression coverage proves the reviewed classifier's false positive and the corrected behavior.
- [ ] Add representative coverage for other supported-dialect result-producing constructs that mutate state without ordinary function-call syntax.
- [ ] Existing safe SELECT/CTE/show-style exports continue to be available, while unknown constructs fail closed.
- [ ] No export correction weakens streaming, cancellation, cap bypass, or atomic-file guarantees from `lazysql-xq1u`.

## Blocked by

None. No implementation prerequisite is required.
