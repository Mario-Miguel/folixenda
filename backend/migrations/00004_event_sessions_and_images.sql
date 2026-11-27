-- Multi-day events and uploaded images.
--
--   - event_sessions: the days of an event (one row per session). The API
--     replaces an event's sessions on every write; events.start_time and
--     end_time stay as the overall span (first session start, last session
--     end), maintained by the API.
--   - events_default_session trigger: the scraper inserts straight into
--     events and knows nothing about sessions, so every inserted event with a
--     start_time gets a default session from start_time/end_time. The API
--     deletes and re-inserts the sessions inside the same transaction, so the
--     trigger's row never survives an API write.
--   - event_images: images uploaded to object storage (MinIO / S3). At most
--     one cover per event; the gallery is ordered by position.
--     NOTE: rows cascade when an event (or its publisher's account) is
--     deleted, but the stored objects don't; the API deletes them when an
--     event is deleted through it.

-- +goose Up
CREATE TABLE IF NOT EXISTS event_sessions (
    id         BIGSERIAL PRIMARY KEY,
    event_id   TEXT      NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    start_time TIMESTAMP NOT NULL,
    end_time   TIMESTAMP NOT NULL,
    UNIQUE (event_id, start_time)
);

CREATE INDEX IF NOT EXISTS event_sessions_start_time_idx ON event_sessions (start_time);

-- One session per existing event.
INSERT INTO event_sessions (event_id, start_time, end_time)
SELECT id, start_time, COALESCE(end_time, start_time)
FROM events
WHERE start_time IS NOT NULL
ON CONFLICT (event_id, start_time) DO NOTHING;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION events_default_session() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.start_time IS NOT NULL THEN
        INSERT INTO event_sessions (event_id, start_time, end_time)
        VALUES (NEW.id, NEW.start_time, COALESCE(NEW.end_time, NEW.start_time))
        ON CONFLICT (event_id, start_time) DO NOTHING;
    END IF;
    RETURN NULL;
END
$$;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS events_default_session ON events;
CREATE TRIGGER events_default_session
    AFTER INSERT ON events
    FOR EACH ROW EXECUTE FUNCTION events_default_session();

CREATE TABLE IF NOT EXISTS event_images (
    id           TEXT        PRIMARY KEY,
    event_id     TEXT        NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    object_key   TEXT        NOT NULL,
    kind         TEXT        NOT NULL CHECK (kind IN ('cover', 'gallery')),
    position     INT         NOT NULL DEFAULT 0,
    content_type TEXT        NOT NULL,
    size_bytes   BIGINT      NOT NULL,
    width        INT,
    height       INT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS event_images_event_id_idx ON event_images (event_id, kind, position);
CREATE UNIQUE INDEX IF NOT EXISTS event_images_one_cover_idx ON event_images (event_id) WHERE kind = 'cover';

-- +goose Down
-- WARNING: loses every event's extra sessions (events keep only their overall
-- start/end span) and the image rows (objects stay in the bucket).
DROP TABLE IF EXISTS event_images;
DROP TRIGGER IF EXISTS events_default_session ON events;
DROP FUNCTION IF EXISTS events_default_session();
DROP TABLE IF EXISTS event_sessions;
