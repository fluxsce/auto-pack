package serverupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func extractArchive(src, dest string) error {
	lower := strings.ToLower(src)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return unzip(src, dest)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return untarGzip(src, dest)
	default:
		return fmt.Errorf("仅支持 .tar.gz / .zip 发行包")
	}
}

func untarGzip(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := writeArchiveEntry(dest, hdr.Name, hdr.FileInfo().IsDir(), hdr.FileInfo().Mode(), tr); err != nil {
			return err
		}
	}
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeArchiveEntry(dest, f.Name, f.FileInfo().IsDir(), f.Mode(), rc)
		_ = rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeArchiveEntry(dest, name string, isDir bool, mode os.FileMode, r io.Reader) error {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == "" {
		return nil
	}
	if strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
		return fmt.Errorf("非法路径 %s", name)
	}
	target := filepath.Join(dest, clean)
	rel, err := filepath.Rel(dest, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("非法路径 %s", name)
	}
	if isDir {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, r)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// payloadRoot 解压后只有一个顶层目录（auto-pack/）时进入该目录。
func payloadRoot(stage string) (string, error) {
	entries, err := os.ReadDir(stage)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, name)
			continue
		}
		// 顶层已有文件，视为 payload 本身。
		return stage, nil
	}
	if len(dirs) == 1 {
		return filepath.Join(stage, dirs[0]), nil
	}
	return stage, nil
}

func findBinary(root, base string) string {
	candidates := []string{
		filepath.Join(root, "bin", base),
		filepath.Join(root, "bin", base+".exe"),
		filepath.Join(root, base),
		filepath.Join(root, base+".exe"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

// mergeTree 把安装包合进 root。跳过运行中的二进制、已有 configs/*.yaml，以及 data/logs/run。
func mergeTree(src, dest, binBase string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		top := parts[0]
		switch top {
		case "data", "logs", "run":
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		base := filepath.Base(path)
		stem := strings.TrimSuffix(base, ".exe")
		if !info.IsDir() && strings.EqualFold(stem, binBase) {
			// 二进制旁路放到 .new，不直接覆盖运行中的文件。
			return nil
		}
		if top == "configs" && !info.IsDir() && strings.HasSuffix(strings.ToLower(base), ".yaml") {
			destFile := filepath.Join(dest, rel)
			if _, err := os.Stat(destFile); err == nil {
				return nil
			}
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}
