package projectcfg

const (
	// DefaultLogRetentionDays is how long build logs are kept when not configured.
	DefaultLogRetentionDays = 3
	minLogRetentionDays     = 1
	maxLogRetentionDays     = 365
)

// EffectiveLogRetentionDays returns a validated retention period in days.
func EffectiveLogRetentionDays(days int) int {
	if days <= 0 {
		return DefaultLogRetentionDays
	}
	if days < minLogRetentionDays {
		return minLogRetentionDays
	}
	if days > maxLogRetentionDays {
		return maxLogRetentionDays
	}
	return days
}
