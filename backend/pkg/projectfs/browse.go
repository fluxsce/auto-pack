package projectfs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/projectcfg"
)

var ErrPathTraversal = errors.New("path escapes project workspace")
var ErrNotFound = errors.New("path not found")
var ErrNotDirectory = errors.New("not a directory")
var ErrFileTooLarge = errors.New("file too large")

const maxReadFileBytes = 512 * 1024

// Entry describes a file or directory under the project work tree.
type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// DirListing is the response for workspace browse.
type DirListing struct {
	Path    string  `json:"path"`
	AbsPath string  `json:"abs_path"`
	Exists  bool    `json:"exists"`
	Entries []Entry `json:"entries"`
}

// FileContent is the response for reading a text file.
type FileContent struct {
	Path      string `json:"path"`
	AbsPath   string `json:"abs_path"`
	Size      int64  `json:"size"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	Readable  bool   `json:"readable"`
}

// ResolveProjectAbsDir returns the absolute project work directory on disk.
func ResolveProjectAbsDir(
	project *repository.PackProject,
	ws *repository.PackWorkspace,
	dataDir string,
) string {
	return projectcfg.JoinWorkDir(ws.RootDir, dataDir, ws.WorkspaceCode, project.WorkDir)
}

// SafeJoin resolves rel under base and rejects path traversal.
func SafeJoin(base, rel string) (string, error) {
	base = filepath.Clean(base)
	if base == "" {
		return "", fmt.Errorf("projectfs: empty base")
	}
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" || rel == "." {
		return base, nil
	}
	rel = strings.TrimPrefix(rel, "/")
	clean := filepath.Clean(filepath.Join(base, rel))
	baseWithSep := base + string(os.PathSeparator)
	if clean != base && !strings.HasPrefix(clean, baseWithSep) {
		return "", ErrPathTraversal
	}
	return clean, nil
}

// ListDir lists children of rel under base.
func ListDir(base, rel string) (DirListing, error) {
	abs, err := SafeJoin(base, rel)
	if err != nil {
		return DirListing{}, err
	}
	displayPath := normalizeDisplayPath(rel)

	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return DirListing{Path: displayPath, AbsPath: abs, Exists: false, Entries: []Entry{}}, nil
		}
		return DirListing{}, err
	}
	if !info.IsDir() {
		return DirListing{}, ErrNotDirectory
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return DirListing{}, err
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		childRel := displayPath
		if childRel == "" || childRel == "." {
			childRel = name
		} else {
			childRel = childRel + "/" + name
		}
		var size int64
		if fi, err := e.Info(); err == nil && !fi.IsDir() {
			size = fi.Size()
		}
		out = append(out, Entry{
			Name:  name,
			Path:  filepath.ToSlash(childRel),
			IsDir: e.IsDir(),
			Size:  size,
		})
	}
	return DirListing{
		Path:    displayPath,
		AbsPath: abs,
		Exists:  true,
		Entries: out,
	}, nil
}

// ReadTextFile reads a file under base with size limit.
func ReadTextFile(base, rel string) (FileContent, error) {
	abs, err := SafeJoin(base, rel)
	if err != nil {
		return FileContent{}, err
	}
	displayPath := normalizeDisplayPath(rel)

	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return FileContent{}, ErrNotFound
		}
		return FileContent{}, err
	}
	if info.IsDir() {
		return FileContent{}, ErrNotDirectory
	}
	if info.Size() > maxReadFileBytes {
		return FileContent{}, ErrFileTooLarge
	}

	f, err := os.Open(abs)
	if err != nil {
		return FileContent{}, err
	}
	defer f.Close()

	b, err := io.ReadAll(io.LimitReader(f, maxReadFileBytes+1))
	if err != nil {
		return FileContent{}, err
	}
	truncated := int64(len(b)) > maxReadFileBytes
	if truncated {
		b = b[:maxReadFileBytes]
	}
	readable := isTextContent(b)
	content := ""
	if readable {
		content = string(b)
	}
	return FileContent{
		Path:      displayPath,
		AbsPath:   abs,
		Size:      info.Size(),
		Content:   content,
		Truncated: truncated,
		Readable:  readable,
	}, nil
}

func normalizeDisplayPath(rel string) string {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return "."
	}
	return rel
}
