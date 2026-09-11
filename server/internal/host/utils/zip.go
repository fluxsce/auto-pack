package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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

// ZipSources 把多个目录打进同一个 dest zip。每个源以自身目录名为根（a/x、b/y）。
// 同名条目后者覆盖。skip 为 true 时跳过该目录或文件。
func ZipSources(srcs []string, dest string, skip ZipSkip) error {
	type pending struct {
		abs string
	}
	files := map[string]pending{}
	dirs := map[string]struct{}{}
	for _, src := range srcs {
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}
		st, err := os.Stat(src)
		if err != nil {
			return err
		}
		rootName := strings.TrimSpace(filepath.Base(src))
		if rootName == "" || rootName == "." {
			return ErrNotDir
		}
		if !st.IsDir() {
			files[rootName] = pending{abs: src}
			continue
		}
		dirs[rootName+"/"] = struct{}{}
		walkErr := filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path != src && skip != nil && skip(d.Name(), d.IsDir()) {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if path == src {
				return nil
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			entry := filepath.ToSlash(filepath.Join(rootName, rel))
			if d.IsDir() {
				dirs[strings.TrimSuffix(entry, "/")+"/"] = struct{}{}
				return nil
			}
			files[entry] = pending{abs: path}
			return nil
		})
		if walkErr != nil {
			return walkErr
		}
	}
	if len(files) == 0 && len(dirs) == 0 {
		return fmt.Errorf("no files to zip")
	}
	return writeZip(dest, func(w *zip.Writer) error {
		dirNames := make([]string, 0, len(dirs))
		for name := range dirs {
			dirNames = append(dirNames, name)
		}
		sort.Strings(dirNames)
		for _, name := range dirNames {
			if _, err := w.Create(name); err != nil {
				return err
			}
		}
		fileNames := make([]string, 0, len(files))
		for name := range files {
			fileNames = append(fileNames, name)
		}
		sort.Strings(fileNames)
		for _, name := range fileNames {
			if err := addFile(w, files[name].abs, name); err != nil {
				return err
			}
		}
		return nil
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
