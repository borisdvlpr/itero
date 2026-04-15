-- Default variant FK (added after variants exists)
ALTER TABLE flags
ADD CONSTRAINT fk_default_variant
FOREIGN KEY (default_variant_id)
REFERENCES variants(id)
ON DELETE SET NULL;

-- Indexes for performance
CREATE INDEX idx_flags_env_key ON flags(environment_id, key);
CREATE INDEX idx_variants_flag ON variants(flag_id);
CREATE INDEX idx_rules_flag_priority ON targeting_rules(flag_id, priority);

-- Ensure deterministic rule ordering per flag
CREATE UNIQUE INDEX idx_rules_flag_priority_unique
ON targeting_rules(flag_id, priority);
