-- +goose Up
CREATE TYPE outbox_status AS ENUM ('pending', 'processing', 'published', 'failed');

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,      -- e.g., "user.created"
    payload BYTEA NOT NULL,        -- Protobuf serialized bytes
    status outbox_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_outbox_events_status_created_at ON outbox_events(status, created_at);

-- +goose Down
DROP TABLE outbox_events;
DROP TYPE outbox_status;
