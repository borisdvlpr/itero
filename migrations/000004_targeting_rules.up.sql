CREATE TABLE targeting_rules (
    id UUID PRIMARY KEY,
    flag_id UUID NOT NULL REFERENCES flags(id) ON DELETE CASCADE,
    priority INTEGER NOT NULL,
    rule_type TEXT NOT NULL,
    conditions JSONB NOT NULL,
    variant_id UUID REFERENCES variants(id) ON DELETE SET NULL,
    rollout_percentage INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
