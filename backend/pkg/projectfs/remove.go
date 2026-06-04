package projectfs

import (
	"os"
)

// RemovePath deletes the file or directory at rel under base (recursive for directories).
// rel "." or empty removes the entire project work directory (base).
func RemovePath(base, rel string) error {
	abs, err := SafeJoin(base, rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return os.RemoveAll(abs)
}
