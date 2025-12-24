package auction

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestValidateBidAmount tests the bid amount validation logic
func TestValidateBidAmount(t *testing.T) {
	tests := []struct {
		name           string
		bidAmount      int64
		currentHighest int64
		wantErr        error
	}{
		{
			name:           "valid bid - higher than current highest",
			bidAmount:      1000,
			currentHighest: 500,
			wantErr:        nil,
		},
		{
			name:           "invalid bid - equal to current highest",
			bidAmount:      500,
			currentHighest: 500,
			wantErr:        ErrBidTooLow,
		},
		{
			name:           "invalid bid - lower than current highest",
			bidAmount:      300,
			currentHighest: 500,
			wantErr:        ErrBidTooLow,
		},
		{
			name:           "valid bid - much higher than current highest",
			bidAmount:      10000,
			currentHighest: 100,
			wantErr:        nil,
		},
		{
			name:           "valid bid - first bid (current highest is 0)",
			bidAmount:      100,
			currentHighest: 0,
			wantErr:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBidAmount(tt.bidAmount, tt.currentHighest)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateAuctionNotEnded tests the auction end time validation logic
func TestValidateAuctionNotEnded(t *testing.T) {
	tests := []struct {
		name    string
		endAt   time.Time
		wantErr error
	}{
		{
			name:    "valid - auction ends in the future",
			endAt:   time.Now().Add(24 * time.Hour),
			wantErr: nil,
		},
		{
			name:    "valid - auction ends far in the future",
			endAt:   time.Now().Add(7 * 24 * time.Hour),
			wantErr: nil,
		},
		{
			name:    "invalid - auction ended 1 hour ago",
			endAt:   time.Now().Add(-1 * time.Hour),
			wantErr: ErrAuctionEnded,
		},
		{
			name:    "invalid - auction ended 1 day ago",
			endAt:   time.Now().Add(-24 * time.Hour),
			wantErr: ErrAuctionEnded,
		},
		{
			name:    "invalid - auction ended just now (1 second ago)",
			endAt:   time.Now().Add(-1 * time.Second),
			wantErr: ErrAuctionEnded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAuctionNotEnded(tt.endAt)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
