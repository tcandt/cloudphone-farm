package middleware

import (
	"net/http/httptest"
	"os"
	"testing"
)

func TestExtractClientIP_UntrustedSpoofRejection(t *testing.T) {
	os.Setenv("TRUSTED_PROXY_CIDR", "192.168.1.0/24")
	defer os.Unsetenv("TRUSTED_PROXY_CIDR")

	// Untrusted peer: 203.0.113.5 (NOT in 192.168.1.0/24)
	req := httptest.NewRequest("GET", "/api/v1/devices", nil)
	req.RemoteAddr = "203.0.113.5:54321"
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	clientIP := ExtractClientIP(req)
	if clientIP != "203.0.113.5" {
		t.Fatalf("Expected client IP 203.0.113.5 from untrusted peer, got: %s", clientIP)
	}
}

func TestExtractClientIP_TrustedProxyHeaderConsumption(t *testing.T) {
	os.Setenv("TRUSTED_PROXY_CIDR", "192.168.1.0/24")
	defer os.Unsetenv("TRUSTED_PROXY_CIDR")

	// Trusted peer: 192.168.1.50 (IN 192.168.1.0/24)
	req := httptest.NewRequest("GET", "/api/v1/devices", nil)
	req.RemoteAddr = "192.168.1.50:54321"
	req.Header.Set("X-Real-IP", "10.0.0.1")

	clientIP := ExtractClientIP(req)
	if clientIP != "10.0.0.1" {
		t.Fatalf("Expected client IP 10.0.0.1 from trusted proxy, got: %s", clientIP)
	}
}
