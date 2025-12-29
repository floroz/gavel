package auth

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
)

type contextKey string

const (
	tokenHeader               = "Authorization"
	tokenPrefix               = "Bearer "
	UserClaimsKey  contextKey = "user_claims"
	UserIDKey      contextKey = "user_id"
	PermissionsKey contextKey = "permissions"
)

// NewAuthInterceptor creates a ConnectRPC interceptor for authentication.
func NewAuthInterceptor(signer *Signer) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			authHeader := req.Header().Get(tokenHeader)
			if authHeader == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing authorization header"))
			}

			if !strings.HasPrefix(authHeader, tokenPrefix) {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid authorization header format"))
			}

			token := strings.TrimPrefix(authHeader, tokenPrefix)
			claims, err := signer.ValidateToken(token)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired token"))
			}

			// Inject info into context
			ctx = context.WithValue(ctx, UserClaimsKey, claims)
			ctx = context.WithValue(ctx, UserIDKey, claims.Subject)
			// We will use this for the permissions check later in Phase 2
			ctx = context.WithValue(ctx, PermissionsKey, claims.Permissions)

			return next(ctx, req)
		}
	}
}

// GetUserClaims retrieves the full claims from the context.
func GetUserClaims(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(UserClaimsKey).(*Claims)
	return claims, ok
}

// GetUserID retrieves the user ID from the context.
func GetUserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(UserIDKey).(string)
	return id, ok
}
