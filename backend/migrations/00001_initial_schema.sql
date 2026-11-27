-- Initial schema for the Go API tables (public schema).
--
-- Requires Better Auth's tables to exist first (schema "auth", table
-- auth."user"): users.id references auth."user"(id). Better Auth creates them
-- with its own migrations (frontend: `npx @better-auth/cli migrate`).
--
-- Every statement is idempotent so this also runs cleanly against databases
-- created before migrations existed (when each Go store created its own table).

-- +goose Up
CREATE TABLE IF NOT EXISTS events (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT,
    category    TEXT NOT NULL,
    start_time  TIMESTAMP,
    end_time    TIMESTAMP,
    venue       TEXT,
    address     TEXT,
    price       DOUBLE PRECISION NOT NULL DEFAULT 0,
    image_url   TEXT,
    artist_name TEXT,
    perks       TEXT[],
    is_saved    BOOLEAN NOT NULL DEFAULT FALSE,
    lat         DOUBLE PRECISION,
    lon         DOUBLE PRECISION
);

-- Same id type as auth."user".id (Better Auth runs with generateId: "uuid").
-- Deleting the Better Auth user deletes this profile and, through the
-- cascades below, everything that belongs to it.
CREATE TABLE IF NOT EXISTS users (
    id                UUID PRIMARY KEY REFERENCES auth."user"(id) ON DELETE CASCADE,
    email             TEXT UNIQUE NOT NULL,
    name              TEXT,
    role              TEXT NOT NULL,
    subscription_type TEXT,
    payment_method    TEXT,
    location          DOUBLE PRECISION[],
    event_preferences TEXT[]
);

CREATE TABLE IF NOT EXISTS publisher_predefined_events (
    id          TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT,
    category    TEXT NOT NULL,
    price       DOUBLE PRECISION NOT NULL DEFAULT 0,
    venue       TEXT,
    image_url   TEXT,
    artist_name TEXT,
    perks       TEXT[]
);

CREATE TABLE IF NOT EXISTS user_saved_events (
    id         TEXT PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id   TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT now(),
    UNIQUE (user_id, event_id)
);

-- Legacy databases: events had a "date" column before start/end timestamps.
-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'events' AND column_name = 'date'
    ) THEN
        UPDATE events
        SET start_time = COALESCE(start_time, date::timestamp),
            end_time   = COALESCE(end_time,   date::timestamp);
        ALTER TABLE events DROP COLUMN date;
    END IF;
END $$;
-- +goose StatementEnd

-- Legacy databases: users had no location column.
ALTER TABLE users ADD COLUMN IF NOT EXISTS location DOUBLE PRECISION[];

-- +goose Down
DROP TABLE IF EXISTS user_saved_events;
DROP TABLE IF EXISTS publisher_predefined_events;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS events;
