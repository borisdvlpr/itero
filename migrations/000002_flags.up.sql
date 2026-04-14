CREATE TYPE flag_type AS ENUM ('boolean', 'string', 'number', 'json');

CREATE TABLE flags (
    id UUID PRIMARY KEY,
    key TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    type flag_type NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    default_variant_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (key, environment_id)
);
