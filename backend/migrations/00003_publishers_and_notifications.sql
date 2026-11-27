-- Two account types: consumers save events, publishers (companies/platforms)
-- publish and manage their own events. Both have notification preferences.
--
--   - publisher_profiles: organization data of a publisher account.
--   - notification_preferences: per-user notification settings (no row means
--     defaults, applied by the API).
--   - events.publisher_id: the publisher that owns the event. NULL for scraped
--     events. Deleting the publisher's account deletes its events.
--   - events.created_at / updated_at: bookkeeping for publisher-managed events.

-- +goose Up
CREATE TABLE IF NOT EXISTS publisher_profiles (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    org_name    TEXT NOT NULL,
    website     TEXT,
    phone       TEXT,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notification_preferences (
    user_id               UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    saved_event_reminders BOOLEAN NOT NULL DEFAULT TRUE,
    category_news         BOOLEAN NOT NULL DEFAULT FALSE,
    categories            TEXT[]  NOT NULL DEFAULT '{}',
    newsletter            BOOLEAN NOT NULL DEFAULT FALSE,
    publisher_activity    BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE events ADD COLUMN IF NOT EXISTS publisher_id UUID NULL REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE events ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE events ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

CREATE INDEX IF NOT EXISTS events_publisher_id_idx ON events (publisher_id);

-- +goose Down
-- WARNING: loses event ownership (publisher_id), publisher profiles and
-- notification preferences.
DROP INDEX IF EXISTS events_publisher_id_idx;
ALTER TABLE events
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS publisher_id;
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS publisher_profiles;
