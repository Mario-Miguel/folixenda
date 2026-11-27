-- Tie public.users to Better Auth's auth."user" so that deleting a Better Auth
-- user also deletes the Go profile and its saved / predefined events.
--
-- Only does real work on databases created before migrations existed (text
-- user ids, foreign keys without ON DELETE CASCADE). On a database created by
-- 00001 it just recreates the same foreign keys.
--
-- WARNING: deletes public.users rows (and their saved events) whose id has no
-- matching auth."user" row, since the foreign key can't exist while they do.
--
-- In production the role running migrations needs, on the auth schema:
--   GRANT USAGE ON SCHEMA auth TO <app role>;
--   GRANT SELECT, REFERENCES, UPDATE (role, name, "updatedAt") ON auth."user" TO <app role>;
-- (UPDATE is used by PUT /api/users/{id} to mirror role/name into Better Auth.)

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    auth_id_type text;
    r record;
BEGIN
    SELECT data_type INTO auth_id_type
    FROM information_schema.columns
    WHERE table_schema = 'auth' AND table_name = 'user' AND column_name = 'id';

    IF auth_id_type IS NULL THEN
        RAISE EXCEPTION 'auth."user" not found: run better_auth.sql and the Better Auth migrations first';
    END IF;
    IF auth_id_type NOT IN ('uuid', 'text') THEN
        RAISE EXCEPTION 'unexpected auth."user".id type: %', auth_id_type;
    END IF;

    -- 1. Drop orphans (compare as text so it works before the type change).
    IF to_regclass('public.user_saved_events') IS NOT NULL THEN
        DELETE FROM public.user_saved_events se
        WHERE NOT EXISTS (SELECT 1 FROM auth."user" a WHERE a.id::text = se.user_id::text);
    END IF;
    IF to_regclass('public.publisher_predefined_events') IS NOT NULL THEN
        DELETE FROM public.publisher_predefined_events p
        WHERE NOT EXISTS (SELECT 1 FROM auth."user" a WHERE a.id::text = p.user_id::text);
    END IF;
    DELETE FROM public.users u
    WHERE NOT EXISTS (SELECT 1 FROM auth."user" a WHERE a.id::text = u.id::text);

    -- 2. Drop every foreign key that will be recreated below (they block the
    --    type change and lack ON DELETE CASCADE).
    FOR r IN
        SELECT c.conrelid::regclass AS tbl, c.conname
        FROM pg_constraint c
        WHERE c.contype = 'f'
          AND (c.conrelid = 'public.users'::regclass
               OR c.confrelid = 'public.users'::regclass
               OR c.conrelid = to_regclass('public.user_saved_events'))
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.tbl, r.conname);
    END LOOP;

    -- 3. Align the id column types with auth."user".id.
    EXECUTE format('ALTER TABLE public.users ALTER COLUMN id TYPE %s USING id::%s',
                   auth_id_type, auth_id_type);
    IF to_regclass('public.user_saved_events') IS NOT NULL THEN
        EXECUTE format('ALTER TABLE public.user_saved_events ALTER COLUMN user_id TYPE %s USING user_id::%s',
                       auth_id_type, auth_id_type);
    END IF;
    IF to_regclass('public.publisher_predefined_events') IS NOT NULL THEN
        EXECUTE format('ALTER TABLE public.publisher_predefined_events ALTER COLUMN user_id TYPE %s USING user_id::%s',
                       auth_id_type, auth_id_type);
    END IF;

    -- 4. Recreate the foreign keys with ON DELETE CASCADE.
    ALTER TABLE public.users
        ADD CONSTRAINT users_id_auth_user_fkey
        FOREIGN KEY (id) REFERENCES auth."user"(id) ON DELETE CASCADE;

    IF to_regclass('public.user_saved_events') IS NOT NULL THEN
        ALTER TABLE public.user_saved_events
            ADD CONSTRAINT user_saved_events_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
        ALTER TABLE public.user_saved_events
            ADD CONSTRAINT user_saved_events_event_id_fkey
            FOREIGN KEY (event_id) REFERENCES public.events(id) ON DELETE CASCADE;
    END IF;
    IF to_regclass('public.publisher_predefined_events') IS NOT NULL THEN
        ALTER TABLE public.publisher_predefined_events
            ADD CONSTRAINT publisher_predefined_events_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Irreversible: converting ids back to text and dropping the cascades would
-- only reintroduce the inconsistencies this migration fixes.
SELECT 1;
