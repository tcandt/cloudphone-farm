package crypto

import (
	"testing"
)

func TestArgon2idPasswordHashingAndVerification(t *testing.T) {
	password := "Pcp_Secure_Pass_2026!#"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if len(hash) == 0 {
		t.Fatal("HashPassword returned empty string")
	}

	// Verify valid password
	match, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if !match {
		t.Errorf("Expected password verification to succeed, got false")
	}

	// Verify wrong password
	matchWrong, err := VerifyPassword("WrongPassword123!", hash)
	if err != nil {
		t.Fatalf("VerifyPassword error on wrong password: %v", err)
	}
	if matchWrong {
		t.Errorf("Expected wrong password verification to fail, got true")
	}

	// Verify malformed hash handling (must not panic!)
	malformedHash := "$argon2id$v=19$m=65536,t=3,p=4$invalid_salt$invalid_hash"
	matchMalformed, err := VerifyPassword(password, malformedHash)
	if err == nil {
		t.Errorf("Expected error on malformed hash, got nil")
	}
	if matchMalformed {
		t.Errorf("Expected malformed hash verification to fail, got true")
	}
}

func TestOpaqueTokenGenerationAndHashing(t *testing.T) {
	token1, err := GenerateOpaqueToken()
	if err != nil {
		t.Fatalf("GenerateOpaqueToken failed: %v", err)
	}

	token2, err := GenerateOpaqueToken()
	if err != nil {
		t.Fatalf("GenerateOpaqueToken failed: %v", err)
	}

	if token1 == token2 {
		t.Errorf("GenerateOpaqueToken produced identical tokens")
	}

	hash1 := HashToken(token1)
	hash1Again := HashToken(token1)
	hash2 := HashToken(token2)

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
