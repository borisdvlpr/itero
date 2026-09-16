CREATE TABLE api_keys
(
    id             UUID PRIMARY KEY,
    project_id     UUID        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    environment_id UUID,
    name           TEXT        NOT NULL,
    key_type       TEXT        NOT NULL CHECK (key_type IN ('server', 'client', 'management')),
    role           TEXT CHECK (role IS NULL OR role IN ('viewer', 'editor', 'admin')),
    token_hash     BYTEA       NOT NULL CHECK (octet_length(token_hash) = 32),
    token_prefix   TEXT        NOT NULL,
    expires_at     TIMESTAMPTZ,
    revoked_at     TIMESTAMPTZ,
    last_used_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (token_hash),
    UNIQUE (project_id, name),
    UNIQUE (token_prefix),
    CONSTRAINT api_keys_environment_matches_type
        CHECK ((key_type IN ('server', 'client')) = (environment_id IS NOT NULL)),
    CONSTRAINT api_keys_role_matches_type
        CHECK ((key_type = 'management') OR role IS NULL),
    FOREIGN KEY (project_id, environment_id)
        REFERENCES environments (project_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_api_keys_active
    ON api_keys (token_hash) WHERE revoked_at IS NULL;

CREATE INDEX idx_api_keys_environment
    ON api_keys (environment_id) WHERE environment_id IS NOT NULL;

CREATE TRIGGER api_keys_set_updated_at
    BEFORE UPDATE
    ON api_keys
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMENT
ON COLUMN api_keys.token_hash IS
    'SHA-256 of the presented key. The plaintext is shown once at creation and '
    'never stored.';

COMMENT
ON COLUMN api_keys.key_type IS
    'server and client are evaluation credentials and must name an '
    'environment; client keys are public by nature and may reach bulk '
    'evaluation only. management is the admin API and is rejected on /ofrep.';