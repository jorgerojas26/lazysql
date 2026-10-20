---
# lazysql-ogdt
title: Add in-chat model and thinking selectors
status: completed
type: feature
priority: normal
created_at: 2026-10-20T21:28:43Z
updated_at: 2026-10-20T21:43:24Z
---

Implement input-border model/thinking controls with Ctrl+P/Ctrl+T and a reusable searchable picker. Use authenticated catalogs and advertised thinking capabilities, persist selections and per-model preferences, keep draft/history/scroll, lock changes while generating, and warn before sending context that may exceed a selected model window.

## Summary of Changes

Added clickable model/thinking labels to the message input border and one responsive searchable picker for both controls. Ctrl+P selects a model; Ctrl+T selects thinking. Added modal-first input handling so Ctrl+T retains global theme behavior outside AI. Catalogs load asynchronously for signed-in providers, group models by provider, preselect the current model, show context windows, and cache within the chat session. Model capability metadata drives supported effort/budget/toggle options without inventing Off or levels.

Persisted model defaults and per-model thinking preferences using scoped config updates, preserving unrelated settings and credential placeholders. Added native provider payload mapping, selection locking during generation, input-focus restoration, draft/history/scroll preservation, and context estimates/warnings with explicit Send anyway and no trimming. Reused model matching in the CLI picker.

Validation: go test ./..., targeted race-enabled selector/catalog/persistence/provider/shortcut tests, go vet ./components ./internal/ai ./app ./internal/aicli, and git diff --check pass. Changes remain uncommitted.
