package workspaceftppush

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxZipBytes = 100 * 1024 * 1024

func zipDirToTemp(absDir string) (tmpPath, zipName string, cleanup func(), err error) {
	rootName := sanitizeZipRoot(filepath.Base(absDir))
	zipName = rootName + ".zip"

	f, err := os.CreateTemp("", "autopack-ftp-push-*.zip")
	if err != nil {
		return "", "", nil, err
	}
	tmpPath = f.Name()
	cleanup = func() { _ = os.Remove(tmpPath) }

	zw := zip.NewWriter(f)
	var total int64
	walkErr := filepath.Walk(absDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if p == absDir {
			return nil
		}
		rel, err := filepath.Rel(absDir, p)
		if err != nil {
			return err
		}
		entryName := filepath.ToSlash(filepath.Join(rootName, rel))
		if info.IsDir() {
			if !strings.HasSuffix(entryName, "/") {
				entryName += "/"
			}
			_, err := zw.Create(entryName)
			return err
		}
		if total+info.Size() > maxZipBytes {
			return fmt.Errorf("directory too large to zip (max %d bytes)", maxZipBytes)
		}
		total += info.Size()
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = entryName
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		src, err := os.Open(p)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(w, src)
		return err
	})
	if closeErr := zw.Close(); closeErr != nil && walkErr == nil {
		walkErr = closeErr
	}
	if closeErr := f.Close(); closeErr != nil && walkErr == nil {
		walkErr = closeErr
	}
	if walkErr != nil {
		cleanup()
		return "", "", nil, walkErr
	}
	return tmpPath, zipName, cleanup, nil
}

func sanitizeZipRoot(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		switch r {
		case '"', '\\', '/', ':', '*', '?', '<', '>', '|':
			return '_'
		default:
			return r
		}
	}, name)
	if name == "" {
		return "dir"
	}
	return name
}
