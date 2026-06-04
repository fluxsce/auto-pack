package envtypes

// DefaultTimeoutSec returns sec or 60 when sec <= 0.
func DefaultTimeoutSec(sec int) int {
	if sec <= 0 {
		return 60
	}
	return sec
}
