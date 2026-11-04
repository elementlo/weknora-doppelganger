CREATE TABLE memory_publications (
 id VARCHAR(64) PRIMARY KEY, tenant_id BIGINT NOT NULL, subject_id VARCHAR(512) NOT NULL,
 publication_key VARCHAR(64) NOT NULL, request_hash VARCHAR(64) NOT NULL,
 payload_hash VARCHAR(64) NOT NULL, item_id VARCHAR(36) NOT NULL, created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_memory_publications_scope ON memory_publications(tenant_id,subject_id);
