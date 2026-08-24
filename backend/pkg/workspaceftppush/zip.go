package workspaceftppush

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FTP push packages directories into a temp zip; shared lib trees often exceed 100MiB.
const maxZipBytes = 2 * 1024 * 1024 * 1024

// DefaultVDatahubZipRoot is the zip file stem (sce-vdatahub.zip) when merging static dirs.
const DefaultVDatahubZipRoot = "sce-vdatahub"

func formatByteSize(n int64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case n >= gib:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(gib))
	case n >= mib:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(mib))
	case n >= kib:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(kib))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

type zipPendingFile struct {
	abs  string
	info os.FileInfo
}

func zipDirToTemp(absDir string) (tmpPath, zipName string, cleanup func(), err error) {
	return zipSourcesToTemp([]string{absDir}, filepath.Base(absDir), nil)
}

func zipSourcesToTemp(absPaths []string, rootName string, logf func(format string, args ...any)) (tmpPath, zipName string, cleanup func(), err error) {
	rootName = sanitizeZipRoot(rootName)
	zipName = rootName + ".zip"

	pending := make(map[string]zipPendingFile)
	dirs := make(map[string]struct{})
	if err := collectZipEntries(absPaths, pending, dirs, logf); err != nil {
		return "", "", nil, err
	}

	f, err := os.CreateTemp("", "autopack-ftp-push-*.zip")
	if err != nil {
		return "", "", nil, err
	}
	tmpPath = f.Name()
	cleanup = func() { _ = os.Remove(tmpPath) }

	zw := zip.NewWriter(f)
	var writeErr error
	dirNames := make([]string, 0, len(dirs))
	for name := range dirs {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)
	for _, name := range dirNames {
		if _, err := zw.Create(name); err != nil {
			writeErr = err
			break
		}
	}

	fileNames := make([]string, 0, len(pending))
	for name := range pending {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	var total int64
	if writeErr == nil {
		for _, name := range fileNames {
			item := pending[name]
			next := total + item.info.Size()
			if next > maxZipBytes {
				writeErr = fmt.Errorf(
					"directory too large to zip (%s uncompressed, max %s)",
					formatByteSize(next),
					formatByteSize(maxZipBytes),
				)
				break
			}
			total = next
			if err := writeZipFile(zw, name, item); err != nil {
				writeErr = err
				break
			}
		}
	}
	if closeErr := zw.Close(); closeErr != nil && writeErr == nil {
		writeErr = closeErr
	}
	if closeErr := f.Close(); closeErr != nil && writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		cleanup()
		return "", "", nil, writeErr
	}
	return tmpPath, zipName, cleanup, nil
}

func collectZipEntries(
	absPaths []string,
	pending map[string]zipPendingFile,
	dirs map[string]struct{},
	logf func(format string, args ...any),
) error {
	for _, abs := range absPaths {
		abs = strings.TrimSpace(abs)
		if abs == "" {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil {
			return err
		}
		entryRoot := sanitizeZipRoot(filepath.Base(abs))
		if !info.IsDir() {
			if prev, ok := pending[entryRoot]; ok && logf != nil {
				logf("Merge overwrite %s (was %s, now %s)", entryRoot, prev.abs, abs)
			}
			pending[entryRoot] = zipPendingFile{abs: abs, info: info}
			continue
		}
		dirs[entryRoot+"/"] = struct{}{}
		walkErr := filepath.Walk(abs, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if p == abs {
				return nil
			}
			rel, err := filepath.Rel(abs, p)
			if err != nil {
				return err
			}
			entryName := filepath.ToSlash(filepath.Join(entryRoot, rel))
			if fi.IsDir() {
				if !strings.HasSuffix(entryName, "/") {
					entryName += "/"
				}
				dirs[entryName] = struct{}{}
				return nil
			}
			if prev, ok := pending[entryName]; ok && logf != nil {
				logf("Merge overwrite %s (was %s, now %s)", entryName, prev.abs, p)
			}
			pending[entryName] = zipPendingFile{abs: p, info: fi}
			return nil
		})
		if walkErr != nil {
			return walkErr
		}
	}
	if len(pending) == 0 && len(dirs) == 0 {
		return fmt.Errorf("no files to zip")
	}
	return nil
}

func writeZipFile(zw *zip.Writer, entryName string, item zipPendingFile) error {
	hdr, err := zip.FileInfoHeader(item.info)
	if err != nil {
		return err
	}
	hdr.Name = entryName
	if isLikelyCompressed(entryName) {
		hdr.Method = zip.Store
	} else {
		hdr.Method = zip.Deflate
	}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	src, err := os.Open(item.abs)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
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

func isLikelyCompressed(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".jar", ".war", ".zip", ".gz", ".tgz", ".7z", ".rar", ".xz", ".bz2"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
