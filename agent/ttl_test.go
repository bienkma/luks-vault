package agent

import (
	"testing"
	"time"
)

func TestIsTTLExpired(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	created := now.Add(-31 * time.Minute).Format(time.RFC3339)

	expired, err := isTTLExpired(created, "30m", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !expired {
		t.Fatal("expected TTL to be expired")
	}
}

func TestIsTTLNotExpired(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	created := now.Add(-10 * time.Minute).Format(time.RFC3339)

	expired, err := isTTLExpired(created, "30m", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expired {
		t.Fatal("expected TTL to still be valid")
	}
}

func TestIsTTLExpiredInvalidCreated(t *testing.T) {
	_, err := isTTLExpired("not-a-time", "30m", time.Now())
	if err == nil {
		t.Fatal("expected error for invalid created timestamp")
	}
}

func TestIsTTLExpiredInvalidTTL(t *testing.T) {
	created := time.Now().Format(time.RFC3339)
	_, err := isTTLExpired(created, "invalid", time.Now())
	if err == nil {
		t.Fatal("expected error for invalid ttl duration")
	}
}

func TestAlternateSlot(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"0", "1"},
		{"1", "0"},
	}

	for _, tc := range tests {
		got, err := alternateSlot(tc.in)
		if err != nil {
			t.Fatalf("slot %q returned error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("slot %q: got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAlternateSlotInvalid(t *testing.T) {
	if _, err := alternateSlot("2"); err == nil {
		t.Fatal("expected error for unsupported slot")
	}
}
