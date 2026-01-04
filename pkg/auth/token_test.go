package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	authv1 "github.com/floroz/gavel/pkg/proto/auth/v1"
)

// Helper to generate fresh keys for each test
func generateTestKeys(t *testing.T) ([]byte, []byte) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return privPEM, pubPEM
}

func TestTokenLifecycle(t *testing.T) {
	privPEM, pubPEM := generateTestKeys(t)
	signer, err := NewSigner(privPEM, pubPEM, "test-issuer")
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}

	userID := uuid.New()
	email := "test@example.com"
	fullName := "Test User"
	permissions := []string{"read:bids"}

	// 1. Generate
	pair, err := signer.GenerateTokens(userID, email, fullName, permissions)
	if err != nil {
		t.Fatalf("GenerateTokens failed: %v", err)
	}

	// 2. Validate
	claims, err := signer.ValidateToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	// 3. Verify Claims
	if claims.Sub != userID.String() {
		t.Errorf("got subject %s, want %s", claims.Sub, userID)
	}
	if claims.Permissions[0] != "read:bids" {
		t.Errorf("got permission %s, want read:bids", claims.Permissions[0])
	}
}

func TestSecurityScenarios(t *testing.T) {
	privPEM, pubPEM := generateTestKeys(t)
	signer, _ := NewSigner(privPEM, pubPEM, "test-issuer")

	// Valid claims for reuse
	validClaims := &Claims{
		TokenClaims: &authv1.TokenClaims{
			Sub:   uuid.New().String(),
			Exp:   float64(time.Now().Add(time.Hour).Unix()),
			Iss:   "gavel-auth-service",
			Email: "hacker@example.com",
		},
	}

	t.Run("Rejects Expired Token", func(t *testing.T) {
		expiredClaims := &Claims{
			TokenClaims: &authv1.TokenClaims{
				Sub:   validClaims.Sub,
				Exp:   float64(time.Now().Add(-1 * time.Hour).Unix()),
				Iss:   validClaims.Iss,
				Email: validClaims.Email,
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, expiredClaims)
		// We need to parse the private key manually to sign this "fake" old token
		block, _ := pem.Decode(privPEM)
		pk, _ := x509.ParsePKCS1PrivateKey(block.Bytes)

		tokenString, _ := token.SignedString(pk)

		_, err := signer.ValidateToken(tokenString)
		if err == nil {
			t.Error("ValidateToken should have rejected expired token")
		}
	})

	t.Run("Rejects Wrong Key Signature", func(t *testing.T) {
		// Generate a DIFFERENT key pair
		attackerPriv, _ := generateTestKeys(t)

		// Sign the token with the ATTACKER'S key
		block, _ := pem.Decode(attackerPriv)
		attackerPK, _ := x509.ParsePKCS1PrivateKey(block.Bytes)

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, validClaims)
		tokenString, _ := token.SignedString(attackerPK)

		// Try to validate with the SERVER'S public key
		_, err := signer.ValidateToken(tokenString)
		if err == nil {
			t.Error("ValidateToken should have rejected token signed by wrong key")
		}
	})

	t.Run("Rejects HMAC Algorithm Confusion", func(t *testing.T) {
		// This simulates an attacker changing "RS256" to "HS256"
		// and signing it with the public key as the secret.
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims)

		// In a real attack, they would use the public key bytes as the HMAC secret.
		// We just want to ensure our validator checks the ALG header.
		tokenString, _ := token.SignedString([]byte("some-secret"))

		_, err := signer.ValidateToken(tokenString)
		if err == nil {
			t.Error("ValidateToken should have rejected HS256 algorithm")
		}
		// The error from jwt.Parse is wrapped, so we check if it contains our specific error message
		expectedError := "unexpected signing method: HS256"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error containing %q, got: %v", expectedError, err)
		}
	})

	t.Run("Rejects Malformed Token", func(t *testing.T) {
		_, err := signer.ValidateToken("this.is.garbage")
		if err == nil {
			t.Error("Should reject malformed string")
		}
	})
}

func TestNewSignerValidation(t *testing.T) {
	_, pubPEM := generateTestKeys(t)

	t.Run("Fails on invalid private key", func(t *testing.T) {
		_, err := NewSigner([]byte("not-a-pem"), pubPEM, "test-issuer")
		if err == nil {
			t.Error("Should fail on invalid private key")
		}
	})
}
