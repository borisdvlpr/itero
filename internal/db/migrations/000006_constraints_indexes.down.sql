DROP INDEX IF EXISTS idx_targeting_rules_variant;
DROP INDEX IF EXISTS idx_flags_default_variant;
DROP INDEX IF EXISTS idx_targeting_rules_flag_priority;

ALTER TABLE flags
    DROP CONSTRAINT IF EXISTS fk_flags_default_variant;