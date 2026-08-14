package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// ComputePublicKeyFingerprint computes a deterministic SHA-256 hex fingerprint from raw public key bytes
func ComputePublicKeyFingerprint(pubKeyBytes []byte) string {
	hash := sha256.Sum256(pubKeyBytes)
	return hex.EncodeToString(hash[:])
}

// GenerateEd25519KeyPair generates a test/agent Ed25519 keypair and SHA-256 fingerprint
func GenerateEd25519KeyPair() (pubKeyB64 string, privKeyB64 string, fingerprint string, err error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate ed25519 keypair: %w", err)
	}

	pubKeyB64 = base64.StdEncoding.EncodeToString(pubKey)
	privKeyB64 = base64.StdEncoding.EncodeToString(privKey)
	fingerprint = ComputePublicKeyFingerprint(pubKey)

	return pubKeyB64, privKeyB64, fingerprint, nil
}
