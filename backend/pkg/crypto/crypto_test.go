package crypto_test

import (
	"testing"

	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

func TestArgon2idPasswordHashingAndVerification(t *testing.T) {
	password := "Pcp_Secure_Pass_2026!#"

	hash, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if len(hash) == 0 {
		t.Fatal("HashPassword returned empty string")
	}

	// Verify valid password
	match, err := crypto.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if !match {
		t.Errorf("Expected password verification to succeed, got false")
	}

	// Verify wrong password
	matchWrong, err := crypto.VerifyPassword("WrongPassword123!", hash)
	if err != nil {
		t.Fatalf("VerifyPassword error on wrong password: %v", err)
	}
	if matchWrong {
		t.Errorf("Expected wrong password verification to fail, got true")
	}

	// Verify malformed hash handling (must not panic!)
	malformedHash := "$argon2id$v=19$m=65536,t=3,p=4$invalid_salt$invalid_hash"
	matchMalformed, err := crypto.VerifyPassword(password, malformedHash)
	if err == nil {
		t.Errorf("Expected error on malformed hash, got nil")
	}
	if matchMalformed {
		t.Errorf("Expected malformed hash verification to fail, got true")
	}
}

func TestOpaqueTokenGenerationAndHashing(t *testing.T) {
	token1, err := crypto.GenerateOpaqueToken()
	if err != nil {
		t.Fatalf("GenerateOpaqueToken failed: %v", err)
	}

	token2, err := crypto.GenerateOpaqueToken()
	if err != nil {
		t.Fatalf("GenerateOpaqueToken failed: %v", err)
	}

	if token1 == token2 {
		t.Errorf("GenerateOpaqueToken produced identical tokens")
	}

	hash1 := crypto.HashToken(token1)
	hash1Again := crypto.HashToken(token1)
	hash2 := crypto.HashToken(token2)

	if hash1 != hash1Again {
		t.Errorf("HashToken is not deterministic")
	}

	if hash1 == hash2 {
		t.Errorf("HashToken produced collision for different tokens")
	}

	if len(hash1) != 64 {
		t.Errorf("Expected SHA-256 hex length 64, got %d", len(hash1))
	}
}

func TestEd25519AgentKeyPairAndFingerprint(t *testing.T) {
	pubB64, privB64, fp, err := crypto.GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateEd25519KeyPair failed: %v", err)
	}

	if pubB64 == "" || privB64 == "" || fp == "" {
		t.Errorf("Generated keypair returned empty values")
	}

	if len(fp) != 64 {
		t.Errorf("Expected SHA-256 fingerprint hex length 64, got %d", len(fp))
	}
}
