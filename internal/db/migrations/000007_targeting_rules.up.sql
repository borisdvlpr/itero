CREATE TABLE targeting_rules
(
    id                  UUID PRIMARY KEY,
    flag_environment_id UUID        NOT NULL
        REFERENCES flag_environments (id) ON DELETE CASCADE,
    priority            INTEGER     NOT NULL CHECK (priority >= 0),
    rule_type           TEXT        NOT NULL CHECK (rule_type IN ('match', 'split')),
    conditions          JSONB       NOT NULL
                                             DEFAULT '{
                                               "version": 1,
                                               "clauses": []
                                             }'::jsonb
                        CHECK (
                            jsonb_typeof(conditions) = 'object'
                            AND COALESCE(jsonb_typeof(conditions -> 'version'), 'absent') = 'number'
                            AND COALESCE(jsonb_typeof(conditions -> 'clauses'), 'absent') = 'array'
                        ),
    variant_id          UUID,
    rollout_percentage  INTEGER CHECK (rollout_percentage IS NULL OR rollout_percentage BETWEEN 0 AND 100),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT targeting_rules_variant_matches_rule_type CHECK ((rule_type = 'match') = (variant_id IS NOT NULL)),
    FOREIGN KEY (flag_environment_id, variant_id) REFERENCES variants (flag_environment_id, id) ON DELETE RESTRICT,
    CONSTRAINT targeting_rules_unique_priority UNIQUE (flag_environment_id, priority) DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX idx_targeting_rules_variant
    ON targeting_rules (flag_environment_id, variant_id);

CREATE TRIGGER targeting_rules_set_updated_at
    BEFORE UPDATE
    ON targeting_rules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER targeting_rules_bump_revision
    AFTER INSERT OR
UPDATE OR
DELETE
ON targeting_rules
    FOR EACH ROW EXECUTE FUNCTION bump_revision_from_flag_environment_child();
