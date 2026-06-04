package envtypes

// DefaultPort returns port or fallback when port <= 0.
func DefaultPort(port, fallback int) int {
	if port <= 0 {
		return fallback
	}
	return port
}
