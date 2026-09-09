package projectfs

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var ErrArchiveTooLarge = errors.New("archive too large")
var ErrNotReadable = errors.New("file is not readable as text")

const maxZipBytes = 100 * 1024 * 1024

// WriteDownload streams a file or zips a directory to w with Content-Disposition headers.
func WriteDownload(w http.ResponseWriter, base, rel string) error {
	abs, err := SafeJoin(base, rel)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	displayPath := normalizeDisplayPath(rel)
	name := filepath.Base(abs)
	if name == "" || name == "." {
		name = "download"
	}

	if info.IsDir() {
		zipName := sanitizeDownloadName(name) + ".zip"
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", contentDispositionAttachment(zipName))
		return writeDirZip(w, abs, displayPath)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", contentDispositionAttachment(sanitizeDownloadName(name)))
	f, err := os.Open(abs)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

func writeDirZip(w io.Writer, absDir, displayRoot string) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	var total int64
	rootName := sanitizeDownloadName(filepath.Base(absDir))
	if rootName == "" {
		rootName = "dir"
	}

	return filepath.Walk(absDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absDir {
			return nil
		}
		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		entryName := filepath.ToSlash(filepath.Join(rootName, rel))

		if info.IsDir() {
			if !strings.HasSuffix(entryName, "/") {
				entryName += "/"
			}
			_, err := zw.Create(entryName)
			return err
		}

		if total+info.Size() > maxZipBytes {
			return ErrArchiveTooLarge
		}
		total += info.Size()

		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = entryName
		hdr.Method = zip.Deflate
		writer, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(writer, f)
		return err
	})
}

func sanitizeDownloadName(name string) string {
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
		return "download"
	}
	return name
}

func contentDispositionAttachment(filename string) string {
	return fmt.Sprintf(`attachment; filename="%s"`, filename)
}

func isTextContent(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	limit := len(b)
	if limit > 8192 {
		limit = 8192
	}
	for _, c := range b[:limit] {
		if c == 0 {
			return false
		}
	}
	return utf8.Valid(b)
}
