package agent

import "time"

const (
	minRotationBackoff = 1 * time.Minute
	maxRotationBackoff = 30 * time.Minute
)

func nextRotationBackoff(current time.Duration) time.Duration {
	if current == 0 {
		return minRotationBackoff
	}

	next := current * 2
	if next > maxRotationBackoff {
		return maxRotationBackoff
	}
	return next
}
