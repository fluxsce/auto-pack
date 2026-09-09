package probe

// Result is the outcome of an environment connectivity or binary probe.
type Result struct {
	OK      bool              `json:"ok"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

func OK(message string, details map[string]string) Result {
	return Result{OK: true, Message: message, Details: details}
}

func Fail(message string, details map[string]string) Result {
	return Result{OK: false, Message: message, Details: details}
}
