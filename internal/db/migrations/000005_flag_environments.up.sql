CREATE TABLE flag_environments
(
    id             UUID PRIMARY KEY,
    project_id     UUID        NOT NULL,
    flag_type      TEXT        NOT NULL,
    flag_id        UUID        NOT NULL,
    environment_id UUID        NOT NULL,
    enabled        BOOLEAN     NOT NULL DEFAULT false,
    version        BIGINT      NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (environment_id, flag_id),
    UNIQUE (id, flag_type),
    FOREIGN KEY (project_id, flag_id)
        REFERENCES flags (project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, environment_id)
        REFERENCES environments (project_id, id) ON DELETE CASCADE,
    FOREIGN KEY (flag_id, flag_type)
        REFERENCES flags (id, type) ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE INDEX idx_flag_environments_flag ON flag_environments (flag_id);

CREATE TRIGGER flag_environments_inherit
    BEFORE INSERT
    ON flag_environments
    FOR EACH ROW EXECUTE FUNCTION inherit_flag_environment_columns();

CREATE TRIGGER flag_environments_touch
    BEFORE UPDATE
    ON flag_environments
    FOR EACH ROW EXECUTE FUNCTION touch_row();

CREATE TRIGGER flag_environments_bump_revision
    AFTER INSERT OR
UPDATE OR
DELETE
ON flag_environments
    FOR EACH ROW EXECUTE FUNCTION bump_revision_from_flag_environment();

COMMENT
ON COLUMN flag_environments.flag_type IS
    'Denormalised from flags.type. Filled on INSERT and maintained by '
    'ON UPDATE CASCADE; never write it directly.';

COMMENT
ON COLUMN flag_environments.version IS
    'Optimistic concurrency token. Without it two concurrent kill-switch edits '
    'silently overwrite one another.';
