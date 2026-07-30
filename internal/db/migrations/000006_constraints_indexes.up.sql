-- Default variant FK (added after variants exists)
ALTER TABLE flags
    ADD CONSTRAINT fk_flags_default_variant
    FOREIGN KEY (default_variant_id)
    REFERENCES variants (id)
    ON DELETE SET NULL;

-- Deterministic rule ordering per flag.
CREATE UNIQUE INDEX idx_targeting_rules_flag_priority
    ON targeting_rules (flag_id, priority);

CREATE INDEX idx_flags_default_variant
    ON flags (default_variant_id)
    WHERE default_variant_id IS NOT NULL;

CREATE INDEX idx_targeting_rules_variant
    ON targeting_rules (variant_id)
    WHERE variant_id IS NOT NULL;
