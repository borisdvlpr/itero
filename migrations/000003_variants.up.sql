CREATE TABLE variants (
    id UUID PRIMARY KEY,
    flag_id UUID NOT NULL REFERENCES flags(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value JSONB NOT NULL,
    weight INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (flag_id, key)
);
