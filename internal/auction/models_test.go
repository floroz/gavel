package auction

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEventType_String tests the String method of EventType
func TestEventType_String(t *testing.T) {
	// Arrange
	eventType := EventTypeBidPlaced

	// Act
	result := eventType.String()

	// Assert
	assert.Equal(t, "bid.placed", result, "EventType.String() should return the correct string representation")
}

// TestEventType_IsValid tests the IsValid method of EventType
func TestEventType_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		want      bool
	}{
		{
			name:      "valid event type - bid.placed",
			eventType: EventTypeBidPlaced,
			want:      true,
		},
		{
			name:      "invalid event type - unknown",
			eventType: EventType("unknown.event"),
			want:      false,
		},
		{
			name:      "invalid event type - empty string",
			eventType: EventType(""),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.eventType.IsValid()
			assert.Equal(t, tt.want, got)
		})
	}
}
