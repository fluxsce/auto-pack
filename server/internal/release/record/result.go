package record

const (
	// ResultOK is a successful project release.
	ResultOK = "ok"
	// ResultSkipped is a project that does not participate in auto-release.
	ResultSkipped = "skipped"
	// ResultFailed is a project that started but did not finish cleanly.
	ResultFailed = "failed"
)

// Result describes the outcome for one project.
type Result struct {
	ProjectName string   `json:"project_name"`
	Status      string   `json:"status"`
	Message     string   `json:"message,omitempty"`
	Steps       []string `json:"steps"`
}
