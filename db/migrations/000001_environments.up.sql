CREATE TABLE environments (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
