package agent

import (
	"testing"
	"time"
)

func TestNextRotationBackoff(t *testing.T) {
	if got := nextRotationBackoff(0); got != minRotationBackoff {
		t.Fatalf("got %v, want %v", got, minRotationBackoff)
	}

	if got := nextRotationBackoff(minRotationBackoff); got != 2*minRotationBackoff {
		t.Fatalf("got %v, want %v", got, 2*minRotationBackoff)
	}

	if got := nextRotationBackoff(20 * time.Minute); got != maxRotationBackoff {
		t.Fatalf("got %v, want %v", got, maxRotationBackoff)
	}
}
