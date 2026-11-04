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

### Host-owned inference (M03.7)

The fork adds extraction/experience `prepare` and `resolve` endpoints. Prepare reuses the memory prompt builder without calling chat or embedding models; resolve reuses the candidate parser. Two-minute process-key HMAC tickets bind authenticated tenant/subject, source content/revision, operation kind and clear generation. Restart invalidates tickets. Existing context is a disclosed importance prefix when it exceeds the bounded extraction window.

`POST /memory/vectors/prepare` returns up to 20 missing-vector texts with scoped content/version tickets; `/vectors/commit` validates ticket, active item, model and configured dimension before storage. `/memory/search/vector` accepts a host-computed query embedding and uses the existing lexical/vector fusion. It returns actual ranking mode, vector hits and skip reason. `host_inference_only` disables service-owned extraction, topic model selection and embedding calls; the workspace pins `host_embedding_dimensions`. User clear advances generation even on an empty subject and retains forget tombstones.

Harness owns gateway credentials, every chat/embedding dispatch, root accounting, truncation retries, shared capacity, deadlines and cancellation. This service owns no second Agent loop or inference pool. In the persistent dedicated instance, real Harness CLI extraction and experience each produced one source-linked candidate; embedding indexed two synthetic notes, and the final search reported hybrid mode with two vector hits. A farther colloquial paraphrase returned no matches, so this is connectivity/lifecycle evidence, not general retrieval-quality acceptance. Related Go memory/repository/handler/router tests and Lite build pass; PostgreSQL-only skipped tests are not live PostgreSQL acceptance.

The fork now has a persistent local deployment for Ddoppelganger Next. `scripts/doppelganger-service.sh` starts the Lite binary with an empty inherited environment and a private `.runtime/.env.lite`. It owns `127.0.0.1:18081`; the old WeKnora service on `127.0.0.1:8080` is a separate application and is not stopped or reused.

The current database is `/Users/element/workspace/ai/weknora-doppelganger/data/weknora.db` (SQLite WAL), local attachments are under `data/files/`, and service logs are under `logs/`. The private account and connection material are under `.runtime/` and are ignored by Git. On 2026-11-03 the persistent instance was migrated to schema 52, a real local account was registered, memory configuration was enabled, one memory item was written and read back, and login plus memory readback remained valid after a service restart. This proves persistence and identity wiring only; it does not prove embedding, automatic extraction, experience distillation, or semantic quality.

On 2026-11-03 the service script was hardened to set the repository working directory before launch, keeping relative SQLite/file paths stable even when invoked elsewhere. PID ownership mismatches fail without discarding the PID file or restarting, restart propagates stop failures, and health failures now produce a nonzero status with a five-second request timeout. `bash -n` passed; calling start/status by absolute path from `/private/tmp` recognized the existing process and returned `{"status":"ok"}` without starting a duplicate. No old service or model was touched.

## 审批发布与持久回执（M03.8）

宿主先保存 Evidence、精确候选预览及本人审批，再通过专用 create-only API 发布：

- `GET /memory/publications/context`：当前主体清除 generation；审批冻结此值。
- `POST /memory/publications`：`key`、`payload_hash`（64 位小写 hex）、`generation`、`kind/content/importance/expires_at`。只在 host_inference_only 开启时允许；显式到期，最多一年。服务拒绝敏感内容改写、过长/非规范正文、重复现存语句、墓碑及容量溢出。没有隐式替代或自动归档。
- `GET /memory/publications/:key`：`state=absent|active|unavailable`，以及 `receipt` 和当前 `item`。身份从已认证 principal 派生，不接受 tenant/subject body 字段。关闭记忆仍可对账历史 receipt，条目不返回可用态。

SQLite 53 / PostgreSQL 137 新增 `memory_publications`。subject 事务同时创建记忆及 receipt，锁定 subject 并复核清除代次；相同 key 只允许完全相同请求。receipt 只保留 key/hash/item ID/时间，不保存候选正文，用户删除/clear 后仍保留，旧请求不能复活内容。每主体上限 10,000，不清理幂等收据腾位置。derived resident block 在事务后重建，失败不否认已提交事实；此接口不触发模型或自动 embedding。

POST 失败可能已提交，宿主只 GET 对账；absent 不证明在途请求永不会提交。receipt 证明历史提交，不能证明当前条目仍可用，宿主还要核对 payload/hash、内容/类型/重要度/到期及当前 Evidence/Grant。完整遗忘穿透备份、产品主体 provision、替代/冲突审批及 UI 尚待实现。

## SQLite 到期时间一致性（M03.9）

显式发布使用 UTC expiry，服务本地时区可能是 +08。SQLite DATETIME 持有带 offset 的文本，直接 `expires_at > ?` 比较会按表面时钟排序，误过滤有效记忆或误保留已过期项。SQLite 的 active 读取、portable vector join 与 overdue 归档现统一用 `julianday` 比较时间点；PostgreSQL 保持原生 timestamp 比较，不改数据库内容和 schema。

回归使用 -12 / +14 offset 检查未来项可读可召回、过去项不可读且仅归档过去项。真实独立数据库中，一小时有效的 UTC 发布记忆经 host embedding 后可被读取，宿主取消来源作业后仍须由领域批准过滤移除；修复时间比较不放宽授权、敏感或召回阈值。
