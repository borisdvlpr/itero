CREATE TABLE audit_log (
    id UUID PRIMARY KEY,
    resource_type TEXT NOT NULL,
    resource_id UUID NOT NULL,
    action TEXT NOT NULL,
    actor TEXT,
    before JSONB,
    after JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
