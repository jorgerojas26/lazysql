---
# lazysql-4zy6
title: '[13] Add AI-driven end-to-end QA harness'
status: completed
type: task
priority: high
created_at: 2026-09-13T03:45:55Z
updated_at: 2026-09-13T04:29:07Z
parent: lazysql-a9lf
---

Implement the first AI-driven end-to-end QA vertical slice for LazySQL on test/ai-qa-harness, based on feat/network-performance. Reuse the existing Docker/manual database fixture environment; do not create another database test environment. Exercise the real LazySQL TUI in a real PTY with Herdr-managed LazySQL and fresh Pi QA-agent panes. The deterministic runner owns setup, builds, timeouts, artifacts, result validation, and suite aggregation; the AI agent owns human-like terminal interaction and semantic oracle judgement.

Acceptance criteria:
- [x] Add qa/README.md, qa/agent-contract.md, qa/suite.json, and the three smoke case instructions under qa/cases/smoke/.
- [x] Add scripts/qa-run.sh with dry-run/validation support, real PTY/Herdr orchestration, one fresh agent per case, machine-readable PASS/FAIL/BLOCKED result validation, evidence under .qa-runs/<run-id>/<case-id>/, cleanup rules, and aggregation.
- [x] Cover only Records pagination (MySQL orders), Foreign Key Jump (seeded relationships), and SQL editor order_items progressive SELECT/truncation cases.
- [x] Verify case instructions against actual keybindings and rendered TUI behavior rather than assumptions.
- [x] Ignore .qa-runs/ and do not add the harness to regular CI.
- [x] Run relevant existing tests plus shell/JSON/static validation and inspect the final diff.
- [x] Append Summary of Changes and complete this bean only after all checklist items are checked.

## Summary of Changes

- Added the real-PTY Herdr/Pi smoke runner with isolated per-run Herdr sessions, per-case workspaces, result validation, evidence capture, cleanup/preservation rules, and suite aggregation.
- Added contract, suite metadata, and three source-checked smoke cases for Records pagination, Foreign Key Jump, and SQL editor streaming/truncation.
- Reused the existing manual database fixtures and local config; added validation/dry-run commands and ignored .qa-runs artifacts.
- Validation: targeted Go tests, go vet, go build, manual fixture validation, QA JSON/shell validation, dry-run, and diff checks passed.
