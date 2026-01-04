package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	authv1 "github.com/floroz/gavel/pkg/proto/auth/v1"
)

// Claims wraps the protobuf TokenClaims to implement jwt.Claims.
type Claims struct {
	*authv1.TokenClaims
}

// Ensure Claims implements jwt.Claims
var _ jwt.Claims = (*Claims)(nil)

func (c *Claims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(int64(c.Exp), 0)), nil
}

func (c *Claims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(int64(c.Iat), 0)), nil
}

func (c *Claims) GetNotBefore() (*jwt.NumericDate, error) {
	return nil, nil
}

func (c *Claims) GetIssuer() (string, error) {
	return c.Iss, nil
}

func (c *Claims) GetSubject() (string, error) {
	return c.Sub, nil
}

func (c *Claims) GetAudience() (jwt.ClaimStrings, error) {
	return nil, nil
}

// TokenPair contains both access and refresh tokens.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	AccessExpiry time.Time
}

// Signer handles token generation and validation.
type Signer struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	issuer     string
}

// NewSigner creates a Signer from PEM-encoded keys (for auth-service that signs tokens).
func NewSigner(privateKeyPEM, publicKeyPEM []byte, issuer string) (*Signer, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, errors.New("failed to parse private key PEM")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	blockPub, _ := pem.Decode(publicKeyPEM)
	if blockPub == nil {
		return nil, errors.New("failed to parse public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(blockPub.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not RSA")
	}

	return &Signer{
		privateKey: priv,
		publicKey:  rsaPub,
		issuer:     issuer,
	}, nil
}

// NewSignerFromPublicKey creates a Signer with only the public key (for services that only validate tokens).
// This signer cannot generate tokens, only validate them.
func NewSignerFromPublicKey(publicKeyPEM []byte, issuer string) (*Signer, error) {
	blockPub, _ := pem.Decode(publicKeyPEM)
	if blockPub == nil {
		return nil, errors.New("failed to parse public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(blockPub.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not RSA")
	}

	return &Signer{
		privateKey: nil, // No private key - cannot sign tokens
		publicKey:  rsaPub,
		issuer:     issuer,
	}, nil
}

// GenerateTokens creates an access token (JWT) and a refresh token (random string).
func (s *Signer) GenerateTokens(userID uuid.UUID, email, fullName string, permissions []string) (*TokenPair, error) {
	now := time.Now()
	accessExpiry := now.Add(15 * time.Minute)

	claims := &Claims{
		TokenClaims: &authv1.TokenClaims{
			Sub:         userID.String(),
			Email:       email,
			FullName:    fullName,
			Permissions: permissions,
			Iss:         s.issuer,
			Exp:         float64(accessExpiry.Unix()),
			Iat:         float64(now.Unix()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	// Generate Refresh Token (32 bytes of entropy)
	refreshToken, err := generateRandomString(32)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  signedToken,
		RefreshToken: refreshToken,
		AccessExpiry: accessExpiry,
	}, nil
}

// ValidateToken parses and verifies the JWT signature.
func (s *Signer) ValidateToken(tokenString string) (*Claims, error) {
	// Initialize with empty TokenClaims to avoid nil pointer panic during unmarshal
	token, err := jwt.ParseWithClaims(tokenString, &Claims{TokenClaims: &authv1.TokenClaims{}}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// We need a helper for generating a secure random string for refresh tokens and other secrets.
// This ensures sufficient entropy and URL-safe characters for security.
func generateRandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
