package auth

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "supersecret123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Error("HashPassword returned empty string")
	}

	// Verify we have a salt and a hash part after the prefix
	// The format is $argon2id$v=19$m=65536,t=1,p=4$SALT$HASH
	parts := strings.Split(hash, "$")
	// Expected parts:
	// [0] "" (before first $)
	// [1] "argon2id"
	// [2] "v=19"
	// [3] "m=65536,t=1,p=4"
	// [4] "SALT" (base64)
	// [5] "HASH" (base64)
	if len(parts) != 6 {
		t.Errorf("Expected 6 parts (including empty start), got %d. Parts: %v", len(parts), parts)
		return
	}

	if parts[1] != "argon2id" {
		t.Errorf("Expected algo 'argon2id', got '%s'", parts[1])
	}

	if parts[2] != "v=19" {
		t.Errorf("Expected version 'v=19', got '%s'", parts[2])
	}

	// Parsing m=65536,t=1,p=4
	params := parts[3]
	if !strings.Contains(params, "m=65536") {
		t.Errorf("Expected memory param m=65536, got params: %s", params)
	}
	if !strings.Contains(params, "t=1") {
		t.Errorf("Expected time param t=1, got params: %s", params)
	}
	if !strings.Contains(params, "p=4") {
		t.Errorf("Expected threads param p=4, got params: %s", params)
	}

	// Verify Salt and Hash are present and look like Base64
	salt := parts[4]
	if len(salt) == 0 {
		t.Error("Salt component is empty")
	}

	hashedKey := parts[5]
	if len(hashedKey) == 0 {
		t.Error("Hashed key component is empty")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "correct-horse-battery-staple"
	wrongPassword := "wrong-password"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// 1. Correct password
	match, err := VerifyPassword(hash, password)
	if err != nil {
		t.Errorf("VerifyPassword error with correct password: %v", err)
	}
	if !match {
		t.Error("VerifyPassword returned false for correct password")
	}

	// 2. Incorrect password
	match, err = VerifyPassword(hash, wrongPassword)
	if err != nil {
		t.Errorf("VerifyPassword error with wrong password: %v", err)
	}
	if match {
		t.Error("VerifyPassword returned true for wrong password")
	}

	// 3. Invalid hash format
	_, err = VerifyPassword("not-a-hash", password)
	if err == nil {
		t.Error("Expected error for invalid hash format, got nil")
	}
}

func TestVerifyPassword_EdgeCases(t *testing.T) {
	// Generate a valid hash to use as a base for tampering
	validHash, _ := HashPassword("password")
	// Split it to easily reconstruct tampered versions
	// Parts: [0]"", [1]"argon2id", [2]"v=19", [3]"m=...,t=...,p=...", [4]SALT, [5]HASH
	parts := strings.Split(validHash, "$")

	tests := []struct {
		name    string
		hash    string
		wantErr bool
	}{
		{
			name:    "Too few parts",
			hash:    "$argon2id$v=19$m=65536,t=1,p=4$salt", // Missing hash part
			wantErr: true,
		},
		{
			name:    "Malformed version (not a number)",
			hash:    "$argon2id$v=xyz$m=65536,t=1,p=4$salt$hash",
			wantErr: true,
		},
		{
			name:    "Incompatible version (v=99)",
			hash:    "$argon2id$v=99$m=65536,t=1,p=4$salt$hash",
			wantErr: true,
		},
		{
			name:    "Malformed parameters (m=abc)",
			hash:    "$argon2id$v=19$m=abc,t=1,p=4$" + parts[4] + "$" + parts[5],
			wantErr: true,
		},
		{
			name:    "Invalid Salt Base64",
			hash:    "$argon2id$v=19$m=65536,t=1,p=4$invalid-salt!$" + parts[5],
			wantErr: true,
		},
		{
			name:    "Invalid Hash Base64",
			hash:    "$argon2id$v=19$m=65536,t=1,p=4$" + parts[4] + "$invalid-hash!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := VerifyPassword(tt.hash, "password")

			// Verify we got an error if we expected one
			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// In all error cases, match should be false
			if match {
				t.Error("Expected match=false, got true")
			}
		})
	}
}
