package auth

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateRefreshToken(t *testing.T) {
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	if raw == "" {
		t.Fatal("GenerateRefreshToken() returned empty raw token")
	}
	if len(hash) != 64 {
		t.Fatalf("refresh token hash length = %d, want 64", len(hash))
	}
	if got := HashRefreshToken(raw); got != hash {
		t.Fatalf("HashRefreshToken(raw) = %q, want %q", got, hash)
	}
	if raw == hash {
		t.Fatal("raw refresh token must not equal its stored hash")
	}
}

func TestManagerIssueAndParse(t *testing.T) {
	manager := NewManager(
		strings.Repeat("s", 32),
		time.Minute,
		"bank-api",
		"bank-client",
	)

	rawToken, expiresAt, err := manager.Issue("user-id")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatal("Issue() returned an already expired token")
	}

	userID, err := manager.Parse(rawToken)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if userID != "user-id" {
		t.Fatalf("Parse() userID = %q, want %q", userID, "user-id")
	}
}
