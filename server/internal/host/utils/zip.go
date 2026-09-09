package utils

import (
	"archive/zip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ZipFile 把单个文件打成 dest zip。
func ZipFile(src, dest string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return ErrNotFile
	}
	return writeZip(dest, func(w *zip.Writer) error {
		return addFile(w, src, filepath.Base(src))
	})
}

// ZipDir 把目录打成 dest zip，条目相对 src。
func ZipDir(src, dest string) error {
	return ZipDirAs(src, dest, "", nil)
}

// ZipSkip 为 true 时跳过该目录（及其子树）或文件。
type ZipSkip func(name string, isDir bool) bool

// ZipDirAs 把目录打成 dest zip。rootName 非空时条目带该前缀（foo/a.txt）。
func ZipDirAs(src, dest, rootName string, skip ZipSkip) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return ErrNotDir
	}
	rootName = strings.TrimSpace(rootName)
	return writeZip(dest, func(w *zip.Writer) error {
		return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path != src && skip != nil && skip(d.Name(), d.IsDir()) {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if rel == "." {
				if rootName != "" {
					_, err := w.Create(strings.TrimSuffix(rootName, "/") + "/")
					return err
				}
				return nil
			}
			if rootName != "" {
				rel = rootName + "/" + rel
			}
			if d.IsDir() {
				_, err := w.Create(strings.TrimSuffix(rel, "/") + "/")
				return err
			}
			return addFile(w, path, rel)
		})
	})
}

func writeZip(dest string, fill func(*zip.Writer) error) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	if err := fill(zw); err != nil {
		_ = zw.Close()
		_ = f.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func addFile(w *zip.Writer, src, name string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := w.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return err
}
