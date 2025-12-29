package auth

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

func TestAuthMiddleware(t *testing.T) {
	privPEM, pubPEM := generateTestKeys(t) // Reusing helper from token_test.go
	signer, _ := NewSigner(privPEM, pubPEM)

	// Generate a valid token
	userID := uuid.New()
	pair, _ := signer.GenerateTokens(userID, "user@example.com", "User", nil)

	interceptor := NewAuthInterceptor(signer)
	dummyHandler := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Verify context injection
		id, ok := GetUserID(ctx)
		if !ok || id != userID.String() {
			t.Errorf("Context missing correct UserID. Got %v, want %s", id, userID)
		}
		return connect.NewResponse(&struct{}{}), nil
	}

	// 1. Test Valid Request
	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Bearer "+pair.AccessToken)

	_, err := interceptor(dummyHandler)(context.Background(), req)
	if err != nil {
		t.Errorf("Unexpected error on valid request: %v", err)
	}

	// 2. Test Missing Header
	reqMissing := connect.NewRequest(&struct{}{})
	_, err = interceptor(dummyHandler)(context.Background(), reqMissing)
	if err == nil {
		t.Error("Expected error for missing header, got nil")
	}

	// 3. Test Invalid Header Format
	reqBadFormat := connect.NewRequest(&struct{}{})
	reqBadFormat.Header().Set("Authorization", pair.AccessToken) // Missing "Bearer "
	_, err = interceptor(dummyHandler)(context.Background(), reqBadFormat)
	if err == nil {
		t.Error("Expected error for bad header format, got nil")
	}
}
