# Harness independent memory API

Status: implementation slice, 2026-11-03. Upstream baseline e3fb2335bf3f8cadd32142380c60b2677a27bf99.

This fork is elementlo/weknora-doppelganger, cloned separately from the existing WeKnora checkout. Develop on codex/harness-memory-api. Existing chat, memory switches, tenant/principal resolution and repository are reused. No second Agent loop is needed.

All routes inherit the personal memory route group's Viewer/full-access API-key policies and derive the memory subject from authenticated context. They reject unknown JSON fields, including arbitrary subject/tenant selection. Bearer credentials are preferred by the current Harness identity handshake.

| Route | Input | Output |
| --- | --- | --- |
| POST /api/v1/memory/search | query (1–2000 runes), limit (1–20) | success/data/available/items; disabled differs from empty |
| POST /api/v1/memory/extractions/preview | sources (1–20), each id/revision/hash/role=user/content (max 1000 runes) | source-linked memory candidates, no automatic write |
| POST /api/v1/memory/experiences/preview | same, role=verified_effect | reusable task candidates; host must verify effects before submitting |

Search delegates to SearchMemory: vector retrieval depends on configured embedding and current engine behavior; do not equate a 200 response with semantic retrieval. Existing engine degrades some failures to empty results; richer diagnostics/error reporting remain needed for final acceptance.

Preview delegates to the existing scoped relevant-memory search, tombstone lookup, prompt/schema, model selection and bounded extraction calls. Only add/update proposals with an explicit valid source index return; delete decisions are not applied. Experience candidates are kind=task. Raw source content is not echoed in returned lineage. Input revision/hash originate from the trusted host and are not independently verified by WeKnora.

Preview is synchronous, has a 60-second request deadline and makes no memory writes; the host must persist/approve candidates and apply through existing CRUD after revalidating sources. It is not the final asynchronous extraction queue API. Return fields model_calls_reserved=2 and completion_tokens_reserved=5200 describe conservative completion ceilings of the reused extractor, not actual usage or total input+output billing. Root budget reservations, input token accounting, persistent jobs/cursors/cancellation and recoverable publication are still required before production integration. Model calls require approved workspace configuration; no gateway secret is bundled.

Verification: go test ./internal/handler ./internal/router ./internal/application/service/memory -count=1 passed. Handler/service tests use synthetic models and SQLite. A built Lite server on 127.0.0.1:18081 with an isolated temporary database passed authenticated synthetic memory create/list/search/update/delete readback using the Harness HTTP client. No embedding was configured; real semantic/model quality is unverified. Original 8080 service and other checkout were untouched.

## Persistent local application setup

The fork now has a persistent local deployment for Ddoppelganger Next. `scripts/doppelganger-service.sh` starts the Lite binary with an empty inherited environment and a private `.runtime/.env.lite`. It owns `127.0.0.1:18081`; the old WeKnora service on `127.0.0.1:8080` is a separate application and is not stopped or reused.

The current database is `/Users/element/workspace/ai/weknora-doppelganger/data/weknora.db` (SQLite WAL), local attachments are under `data/files/`, and service logs are under `logs/`. The private account and connection material are under `.runtime/` and are ignored by Git. On 2026-11-03 the persistent instance was migrated to schema 52, a real local account was registered, memory configuration was enabled, one memory item was written and read back, and login plus memory readback remained valid after a service restart. This proves persistence and identity wiring only; it does not prove embedding, automatic extraction, experience distillation, or semantic quality.

On 2026-11-03 the service script was hardened to set the repository working directory before launch, keeping relative SQLite/file paths stable even when invoked elsewhere. PID ownership mismatches fail without discarding the PID file or restarting, restart propagates stop failures, and health failures now produce a nonzero status with a five-second request timeout. `bash -n` passed; calling start/status by absolute path from `/private/tmp` recognized the existing process and returned `{"status":"ok"}` without starting a duplicate. No old service or model was touched.
