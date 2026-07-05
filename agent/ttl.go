package agent

import (
	"fmt"
	"time"
)

func isTTLExpired(created, ttl string, now time.Time) (bool, error) {
	createdAt, err := time.Parse(time.RFC3339, created)
	if err != nil {
		return false, fmt.Errorf("parse created timestamp: %w", err)
	}

	duration, err := time.ParseDuration(ttl)
	if err != nil {
		return false, fmt.Errorf("parse ttl duration: %w", err)
	}
	if duration <= 0 {
		return false, fmt.Errorf("ttl must be greater than zero")
	}

	return createdAt.Add(duration).Before(now), nil
}

func alternateSlot(slot string) (string, error) {
	switch slot {
	case "0":
		return "1", nil
	case "1":
		return "0", nil
	default:
		return "", fmt.Errorf("unsupported LUKS key slot %q, expected 0 or 1", slot)
	}
}
