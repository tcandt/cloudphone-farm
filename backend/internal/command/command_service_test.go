package command

import (
	"testing"
)

func TestParseNumber(t *testing.T) {
	num, ok := parseNumber(0.5)
	if !ok || num != 0.5 {
		t.Fatalf("expected 0.5, got %v", num)
	}

	numInt, okInt := parseNumber(100)
	if !okInt || numInt != 100.0 {
		t.Fatalf("expected 100.0, got %v", numInt)
	}

	_, okStr := parseNumber("invalid")
	if okStr {
		t.Fatalf("expected string parsing to fail")
	}
}

func TestComparePayloadFingerprint(t *testing.T) {
	existingBytes := []byte(`{
		"x": 0.5,
		"y": 0.3,
		"coordinateSpace": "normalized_display_v1",
		"orientation": "portrait",
		"control_lease_id": "lease_123",
		"fencing_token": 1
	}`)

	reqPayloadSame := map[string]interface{}{
		"x":               0.5,
		"y":               0.3,
		"coordinateSpace": "normalized_display_v1",
		"orientation":     "portrait",
	}

	if !comparePayloadFingerprint(existingBytes, reqPayloadSame) {
		t.Fatalf("expected payload fingerprint comparison to succeed for identical user payload")
	}

	reqPayloadDiff := map[string]interface{}{
		"x":               0.8,
		"y":               0.3,
		"coordinateSpace": "normalized_display_v1",
		"orientation":     "portrait",
	}

	if comparePayloadFingerprint(existingBytes, reqPayloadDiff) {
		t.Fatalf("expected payload fingerprint comparison to fail for different coordinates")
	}
}
