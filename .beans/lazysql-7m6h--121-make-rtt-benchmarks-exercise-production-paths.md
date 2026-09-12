---
# lazysql-7m6h
title: '[12.1] Make RTT benchmarks exercise production paths'
status: todo
type: bug
priority: high
tags:
    - ready-for-agent
    - order-12-01
    - review-finding
    - review-cycle-0002
created_at: 2026-09-12T20:42:44Z
updated_at: 2026-09-12T20:42:44Z
parent: lazysql-a9lf
---

## Review source

Review of `v0.5.7...HEAD` (fixed point `a07385a07523428234e350727f9e769f464e302e`, pinned HEAD `8bdaefccbb191f33a5285f7f25bec1a41b1b1c03`).

Owning Bean: `lazysql-irgk — [12] Lock in the network-performance contract with benchmarks, logs, and docs`

Axes:
- Standards: none
- Spec: The RTT benchmark matrix models desired counters without exercising the production implementation whose regressions it is intended to detect.

## Observed behavior

The benchmark harness hard-codes simulated operation counts, rows, and bytes for Records, MySQL #340, streaming, autocomplete, and export. It does not invoke the production driver/component/schema-loader/export paths. A regression that adds blocking database operations, breaks streaming/caps, restores the pathological FK path, or materializes export can leave every benchmark metric unchanged and green.

This leaves `lazysql-irgk`'s benchmark/integration harness and regression-resistant acceptance-matrix requirements only partially implemented.

## What to build

Make the credential-free RTT matrix measure observable work produced by the real implementation boundaries, using deterministic instrumented database/driver collaborators where external services are inappropriate. Scenario metrics must derive from executed production behavior rather than scenario-authored expected counters.

## Acceptance criteria

- [ ] RTT delay and operation measurements are injected at production database/driver boundaries used by the reviewed feature paths, not represented solely by hard-coded scenario counters.
- [ ] Records, pagination/filtering, autocomplete threshold cases, SQL cap/slow rows, full export, and MySQL #340 scenarios exercise the relevant production orchestration or driver logic.
- [ ] Reported blocking round trips, total operations, rows, bytes, TTFUR, and background completion are derived from observed scenario execution.
- [ ] Regression coverage demonstrates that an extra blocking call or equivalent contract break changes/fails the matrix instead of remaining green.
- [ ] The matrix remains deterministic, credential-free, and supports 0 ms, 50 ms, and 100 ms artificial RTT without fragile wall-clock pass/fail thresholds.
- [ ] Preserve the existing benchmark CLI/output contract and deterministic CI performance-contract tests.

## Blocked by

None. No implementation prerequisite is required.
