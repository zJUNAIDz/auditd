CREATE TABLE audit_events (
    id            UUID        PRIMARY KEY,
    tenant_id     UUID        NOT NULL REFERENCES tenants(id),
    actor_id      TEXT        NOT NULL,
    actor_type    TEXT        NOT NULL CHECK (actor_type IN ('user', 'service', 'system')),
    action        TEXT        NOT NULL,
    resource_type TEXT        NOT NULL,
    resource_id   TEXT        NOT NULL,
    metadata      JSONB       NOT NULL DEFAULT '{}',
    timestamp     TIMESTAMPTZ NOT NULL,
    prev_hash     TEXT        NOT NULL,
    hash          TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_events_tenant_ts ON audit_events (tenant_id, timestamp DESC);
CREATE INDEX idx_audit_events_tenant_action ON audit_events (tenant_id, action);

-- Append-only enforcement at the DB level
CREATE RULE no_update AS ON UPDATE TO audit_events DO INSTEAD NOTHING;
CREATE RULE no_delete AS ON DELETE TO audit_events DO INSTEAD NOTHING;

CREATE INDEX idx_audit_events_actor_id ON audit_events (actor_id);
CREATE INDEX idx_audit_events_resource_id ON audit_events (resource_id);
CREATE INDEX idx_audit_events_created_at ON audit_events (created_at);
CREATE INDEX idx_audit_events_tenant_id ON audit_events (tenant_id);