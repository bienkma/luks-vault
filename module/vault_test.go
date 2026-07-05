package module

import (
	"testing"
	"time"
)

func TestParseVaultData(t *testing.T) {
	data, err := parseVaultData(map[string]interface{}{
		"key":     "secret-key",
		"ttl":     "30m",
		"slot":    "1",
		"created": "2026-07-05T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Key != "secret-key" || data.TTL != "30m" || data.Slot != "1" {
		t.Fatalf("unexpected parsed data: %+v", data)
	}
}

func TestParseVaultDataCreatedTime(t *testing.T) {
	created := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	data, err := parseVaultData(map[string]interface{}{
		"key":     "secret-key",
		"ttl":     "30m",
		"slot":    "0",
		"created": created,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Created != created.Format(time.RFC3339) {
		t.Fatalf("got created %q, want %q", data.Created, created.Format(time.RFC3339))
	}
}

func TestParseVaultDataMissingField(t *testing.T) {
	_, err := parseVaultData(map[string]interface{}{
		"key":  "secret-key",
		"ttl":  "30m",
		"slot": "0",
	})
	if err == nil {
		t.Fatal("expected error for missing created field")
	}
}

func TestParseVaultDataInvalidFieldType(t *testing.T) {
	_, err := parseVaultData(map[string]interface{}{
		"key":     123,
		"ttl":     "30m",
		"slot":    "0",
		"created": "2026-07-05T12:00:00Z",
	})
	if err == nil {
		t.Fatal("expected error for invalid key type")
	}
}
