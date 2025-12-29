-- +goose Up
CREATE TABLE items (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    start_price BIGINT NOT NULL CHECK (start_price >= 0),
    current_highest_bid BIGINT DEFAULT 0 CHECK (current_highest_bid >= 0),
    end_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE bids (
    id UUID PRIMARY KEY,
    item_id UUID NOT NULL REFERENCES items(id),
    user_id UUID NOT NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- +goose Down
DROP TABLE bids;
DROP TABLE items;

