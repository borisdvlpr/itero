-- Shared functions used by triggers throughout the schema. They are created
-- before any table so that every migration below can attach its triggers
-- without ordering constraints: a PL/pgSQL body is only syntax-checked at
-- CREATE time, so it may reference a table that does not exist yet.

-- ---------------------------------------------------------------------------
-- Timestamps and row versions
-- ---------------------------------------------------------------------------

-- set_updated_at maintains updated_at on tables that carry no version column.
CREATE FUNCTION set_updated_at() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

-- touch_row maintains updated_at and increments version on every UPDATE, so
-- optimistic concurrency is enforced by the database rather than by convention.
-- Callers guard with `WHERE id = $1 AND version = $2` and treat a zero row
-- count as a lost race. A version supplied by the caller is ignored.
CREATE FUNCTION touch_row() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    NEW.version    := OLD.version + 1;
    RETURN NEW;
END;
$$;

-- ---------------------------------------------------------------------------
-- Environment configuration revision
-- ---------------------------------------------------------------------------

-- bump_environments raises the configuration revision of each listed
-- environment and announces the new value on the itero_flag_config channel.
--
-- revision answers three questions with one number: what ETag a bulk
-- evaluation response should carry, whether an incoming If-None-Match can be
-- answered with 304, and when an in-memory evaluation snapshot is stale. It is
-- maintained here rather than in application code so that no write path can
-- forget to move it.
--
-- Bumping takes a row lock on the environment, so concurrent writes to the same
-- environment serialise. At management-API write volumes that is not a
-- bottleneck, and it is what makes the ordering meaningful.
CREATE FUNCTION bump_environments(env_ids UUID[]) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    bumped RECORD;
BEGIN
    FOR bumped IN
        UPDATE environments
           SET revision = revision + 1
         WHERE id = ANY(env_ids)
        RETURNING id, revision
    LOOP
        PERFORM pg_notify(
            'itero_flag_config',
            json_build_object(
                'environment_id', bumped.id,
                'revision', bumped.revision
            )::text
        );
    END LOOP;
END;
$$;

-- A change to a logical flag is visible in every environment where the flag is
-- enrolled. DELETE is not handled here: removing a flag cascades to
-- flag_environments, whose own trigger bumps each affected environment.
CREATE FUNCTION bump_revision_from_flag() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    affected UUID[];
BEGIN
    SELECT array_agg(environment_id)
      INTO affected
      FROM flag_environments
     WHERE flag_id = NEW.id;

    IF affected IS NOT NULL THEN
        PERFORM bump_environments(affected);
    END IF;

    RETURN NULL;
END;
$$;

-- A flag_environment names its environment directly.
CREATE FUNCTION bump_revision_from_flag_environment() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM bump_environments(ARRAY[OLD.environment_id]);
    ELSE
        PERFORM bump_environments(ARRAY[NEW.environment_id]);
    END IF;

    RETURN NULL;
END;
$$;

-- Variants and targeting rules reach their environment through
-- flag_environments. When the parent has already gone -- a cascading delete --
-- the lookup finds nothing and the parent's own trigger has done the bump.
CREATE FUNCTION bump_revision_from_flag_environment_child() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    parent     UUID;
    target_env UUID;
BEGIN
    IF TG_OP = 'DELETE' THEN
        parent := OLD.flag_environment_id;
    ELSE
        parent := NEW.flag_environment_id;
    END IF;

    SELECT environment_id
      INTO target_env
      FROM flag_environments
     WHERE id = parent;

    IF FOUND THEN
        PERFORM bump_environments(ARRAY[target_env]);
    END IF;

    RETURN NULL;
END;
$$;

-- ---------------------------------------------------------------------------
-- Flag type propagation
-- ---------------------------------------------------------------------------
--
-- flag_environments.flag_type and variants.flag_type are denormalised copies of
-- flags.type. They exist so that "a variant's JSON value matches its flag's
-- declared type" can be a CHECK constraint instead of application discipline,
-- and composite foreign keys with ON UPDATE CASCADE keep them honest. These two
-- triggers fill the columns on INSERT so that no caller has to know they are
-- there.

CREATE FUNCTION inherit_flag_environment_columns() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    SELECT type, project_id
      INTO NEW.flag_type, NEW.project_id
      FROM flags
     WHERE id = NEW.flag_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'flag % does not exist', NEW.flag_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    RETURN NEW;
END;
$$;

CREATE FUNCTION inherit_variant_columns() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
BEGIN
    SELECT flag_type
      INTO NEW.flag_type
      FROM flag_environments
     WHERE id = NEW.flag_environment_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'flag_environment % does not exist', NEW.flag_environment_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    RETURN NEW;
END;
$$;

-- ---------------------------------------------------------------------------
-- Variant weights
-- ---------------------------------------------------------------------------

-- check_variant_weights enforces the invariant no single-row CHECK can see:
-- within one flag_environment, weight is set on every variant or on none, and
-- when it is set the weights sum to 100. It is attached as a DEFERRABLE
-- INITIALLY DEFERRED constraint trigger so a transaction may pass through
-- invalid intermediate states while rewriting a split.
CREATE FUNCTION check_variant_weights() RETURNS TRIGGER
LANGUAGE plpgsql AS $$
DECLARE
    parent     UUID;
    variants_n INTEGER;
    weighted_n INTEGER;
    weight_sum BIGINT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        parent := OLD.flag_environment_id;
    ELSE
        parent := NEW.flag_environment_id;
    END IF;

    SELECT count(*), count(weight), COALESCE(sum(weight), 0)
      INTO variants_n, weighted_n, weight_sum
      FROM variants
     WHERE flag_environment_id = parent;

    -- No split configured, or the whole flag_environment has been removed.
    IF weighted_n = 0 THEN
        RETURN NULL;
    END IF;

    IF weighted_n <> variants_n THEN
        RAISE EXCEPTION
            'flag_environment %: weight is set on % of % variants; set it on all of them or on none',
            parent, weighted_n, variants_n
            USING ERRCODE = 'check_violation';
    END IF;

    IF weight_sum <> 100 THEN
        RAISE EXCEPTION
            'flag_environment %: variant weights sum to %, expected 100',
            parent, weight_sum
            USING ERRCODE = 'check_violation';
    END IF;

    RETURN NULL;
END;
$$;
