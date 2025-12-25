-- +goose Up
CREATE TABLE user_stats (
    user_id UUID PRIMARY KEY,
    total_bids_placed BIGINT NOT NULL DEFAULT 0,
    total_amount_bid BIGINT NOT NULL DEFAULT 0,
    last_bid_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE processed_events (
    event_id UUID PRIMARY KEY,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- +goose Down
DROP TABLE processed_events;
DROP TABLE user_stats;

