package git

import "testing"

func TestIsRecoverableSyncError_pathspec(t *testing.T) {
	err := errorString("checkout flux: git checkout develop: error: pathspec 'origin/develop' did not match any file(s) known to git.")
	if !IsRecoverableSyncError(err) {
		t.Fatal("pathspec checkout failure should be recoverable")
	}
	if IsRecoverableSyncError(errorString("git fetch: authentication failed")) {
		t.Fatal("auth failure should not be recoverable")
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }
