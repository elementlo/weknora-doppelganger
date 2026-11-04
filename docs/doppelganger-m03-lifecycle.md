# Doppelganger M03 host lifecycle extensions

The dedicated Doppelganger instance runs on 127.0.0.1:18081 in Lite mode. These endpoints retain the authenticated tenant and subject; request data cannot select another principal. The host remains the sole agent and inference scheduler.

`POST /api/v1/memory/search/vector` accepts optional `item_ids` (at most 200 unique IDs). When present, it fetches only active unexpired items in this authenticated subject before lexical/vector ranking; an empty list yields no items. Host-approved IDs are mandatory for product recall. Omitting the field preserves the trusted diagnostic search API. Query vectors pin model ID and dimensions. Clear generation is rechecked after search.

`POST /api/v1/memory/publications/:key/forget` accepts only `payload_hash`. The subject transaction checks the receipt hash, deletes the exact item and embedding, and preserves a content-free receipt. If the original publication has not arrived, an erased receipt is created first. A late publication with this key is rejected, including after host retry. This avoids the absence-then-late-POST resurrection window. Rebuilding the resident block follows the transaction; read APIs observe no item after deletion.

Validation: `go test -tags sqlite_fts5 ./internal/application/service/memory ./internal/application/repository ./internal/handler`. New regressions cover empty/duplicate approval filters, pre-ranking allowlists, payload mismatch, repeated forget, cross-tenant separation, deleted embeddings and late-create fencing. The consuming app's M03 report records real HTTP/model acceptance with synthetic data. No changes apply to the unrelated original 8080 instance.

Host inference preparation additionally uses only the explicitly supplied sources (`existing_coverage: host_sources_only`). It does not include subject-wide existing notes, rejected-note excerpts or topics: one service principal may serve multiple product scopes. Deduplication/conflict review belongs to the host's scoped domain approval. A canary regression proves another scope's stored note never enters the extraction prompt.
