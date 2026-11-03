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
