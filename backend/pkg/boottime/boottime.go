package boottime

import "time"

var startedAt = time.Now().UTC()

// StartedAt is the process start time (changes after each server restart).
func StartedAt() time.Time {
	return startedAt
}

// StartedAtRFC3339Nano returns StartedAt as RFC3339Nano for API clients.
func StartedAtRFC3339Nano() string {
	return startedAt.UTC().Format(time.RFC3339Nano)
}
