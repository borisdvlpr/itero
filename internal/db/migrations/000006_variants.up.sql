CREATE TABLE variants
(
    id                  UUID PRIMARY KEY,
    flag_environment_id UUID NOT NULL,
    flag_type           TEXT NOT NULL,
    key                 TEXT NOT NULL CHECK (key ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$'),
    value               JSONB NOT NULL,
    is_default          BOOLEAN NOT NULL DEFAULT false,
    weight              INTEGER
                        CHECK (weight IS NULL OR weight BETWEEN 0 AND 100),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (flag_environment_id, key),
    UNIQUE (flag_environment_id, id),
    FOREIGN KEY (flag_environment_id, flag_type)
        REFERENCES flag_environments (id, flag_type)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT variants_value_matches_flag_type CHECK (
        CASE flag_type
            WHEN 'boolean' THEN jsonb_typeof(value) = 'boolean'
            WHEN 'string'  THEN jsonb_typeof(value) = 'string'
            WHEN 'float'   THEN jsonb_typeof(value) = 'number'
            WHEN 'object'  THEN jsonb_typeof(value) = 'object'
            WHEN 'integer' THEN jsonb_typeof(value) = 'number'
                            AND (value #>> '{}') ~ '^-?[0-9]+(\.0+)?$'
            ELSE false
        END
    )
);

CREATE UNIQUE INDEX idx_variants_one_default
    ON variants (flag_environment_id) WHERE is_default;

CREATE TRIGGER variants_inherit
    BEFORE INSERT
    ON variants
    FOR EACH ROW EXECUTE FUNCTION inherit_variant_columns();

CREATE TRIGGER variants_set_updated_at
    BEFORE UPDATE
    ON variants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER variants_bump_revision
    AFTER INSERT OR
UPDATE OR
DELETE
ON variants
    FOR EACH ROW EXECUTE FUNCTION bump_revision_from_flag_environment_child();

CREATE
CONSTRAINT TRIGGER variants_weights_sum_to_100
    AFTER INSERT OR
UPDATE OR
DELETE
ON variants
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION check_variant_weights();

COMMENT
ON COLUMN variants.flag_type IS
    'Denormalised from flags.type via flag_environments. Filled on INSERT and '
    'maintained by ON UPDATE CASCADE; never write it directly.';
