package projectrelease

import (
	"strings"
	"testing"

	"auto-pack/backend/pkg/shell"
)

func TestCommitResultErr(t *testing.T) {
	err := commitResultErr("commit", shell.Result{ExitCode: 1, Stderr: "nothing to commit"}, nil)
	if !strings.Contains(err.Error(), "nothing to commit") {
		t.Fatalf("got %q", err.Error())
	}
	if strings.Contains(err.Error(), "git.exe") {
		t.Fatalf("should not embed shell command label: %q", err.Error())
	}
}
