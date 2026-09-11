package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/server/internal/host/utils"
	"auto-pack/server/internal/iam"
)

const (
	maxTreeEntries = 400
	maxFileBytes   = 256 << 10
)

var (
	// ErrOutsideCheckout 请求路径跳出了工程检出目录。
	ErrOutsideCheckout = utils.ErrOutside
	// ErrNotFile 目标不是普通文件。
	ErrNotFile = utils.ErrNotFile
	// ErrNotDir 目标不是目录。
	ErrNotDir = utils.ErrNotDir
	// ErrNotText 目标不是 UTF-8 文本。
	ErrNotText = utils.ErrNotText
)

// FileEntry 是检出目录下一层的文件或子目录。
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size,omitempty"`
	ModTime string `json:"mod_time,omitempty"`
}

// FileBlob 是检出目录内一个文本文件的内容。
type FileBlob struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Tree 是一次目录列举的结果。
type Tree struct {
	Root    string      `json:"root"`
	Path    string      `json:"path"`
	Exists  bool        `json:"exists"`
	Entries []FileEntry `json:"entries"`
}

// ListCheckout 列举工程检出目录下的一层文件。rel 相对工程目录，禁止 ..。
func (s *Service) ListCheckout(ctx context.Context, actor iam.Actor, workspaceID, workRel, rel, absLocal string) (Tree, error) {
	base, rel, err := s.checkoutPath(ctx, actor, workspaceID, workRel, rel, absLocal)
	if err != nil {
		return Tree{}, err
	}
	exists, items, err := utils.ListDir(base, rel, maxTreeEntries)
	if err != nil {
		return Tree{}, err
	}
	out := Tree{Root: base, Path: rel, Exists: exists, Entries: make([]FileEntry, 0, len(items))}
	for _, item := range items {
		out.Entries = append(out.Entries, FileEntry{
			Name:    item.Name,
			Path:    item.Path,
			IsDir:   item.IsDir,
			Size:    item.Size,
			ModTime: item.ModTime,
		})
	}
	return out, nil
}

// ReadCheckout 读取工程检出目录内的文本文件。
func (s *Service) ReadCheckout(ctx context.Context, actor iam.Actor, workspaceID, workRel, rel, absLocal string) (FileBlob, error) {
	base, rel, err := s.checkoutPath(ctx, actor, workspaceID, workRel, rel, absLocal)
	if err != nil {
		return FileBlob{}, err
	}
	text, truncated, err := utils.ReadText(base, rel, maxFileBytes)
	if err != nil {
		return FileBlob{}, err
	}
	return FileBlob{Path: rel, Content: text, Truncated: truncated}, nil
}

// CheckoutTarget 是检出目录内一个已解析的文件或子目录。
type CheckoutTarget struct {
	Abs  string
	Rel  string
	Name string
	Dir  bool
}

// StatCheckout 解析工程检出目录内的文件或子目录。rel 不能为空。
func (s *Service) StatCheckout(ctx context.Context, actor iam.Actor, workspaceID, workRel, rel, absLocal string) (CheckoutTarget, error) {
	base, rel, err := s.checkoutPath(ctx, actor, workspaceID, workRel, rel, absLocal)
	if err != nil {
		return CheckoutTarget{}, err
	}
	if rel == "" {
		return CheckoutTarget{}, ErrOutsideCheckout
	}
	abs := utils.JoinUnder(base, rel)
	st, err := os.Stat(abs)
	if err != nil {
		return CheckoutTarget{}, err
	}
	return CheckoutTarget{Abs: abs, Rel: rel, Name: filepath.Base(abs), Dir: st.IsDir()}, nil
}

// RemoveCheckout 删除工程检出目录内的文件或子目录。需要 project.write。
// 禁止删除工程根、工作空间根或配置的 workspace.root。
func (s *Service) RemoveCheckout(ctx context.Context, actor iam.Actor, workspaceID, workRel, rel, absLocal string) error {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionProjectWrite); err != nil {
		return err
	}
	base, rel, err := s.checkoutPath(ctx, actor, workspaceID, workRel, rel, absLocal)
	if err != nil {
		return err
	}
	if rel == "" {
		return ErrOutsideCheckout
	}
	dest := utils.JoinUnder(base, rel)
	if filepath.Clean(dest) == filepath.Clean(s.root) {
		return ErrOutsideCheckout
	}
	w, err := s.getForActor(ctx, actor, workspaceID)
	if err != nil {
		return err
	}
	wsDir := Dir(s.root, w.WorkspaceCode, w.RecID)
	if filepath.Clean(dest) == filepath.Clean(wsDir) || filepath.Clean(dest) == filepath.Clean(base) {
		return ErrOutsideCheckout
	}
	return utils.RemoveRel(base, rel)
}

// SkipNestedGit 打包目录时跳过嵌套的 .git，避免把对象库打进下载包。
func SkipNestedGit(name string, isDir bool) bool {
	return isDir && name == ".git"
}

// ClearCheckout 清空工程检出目录内的文件，保留目录本身。需要 project.write。
// 禁止清空工作空间根或配置的 workspace.root。
func (s *Service) ClearCheckout(ctx context.Context, actor iam.Actor, workspaceID, workRel, absLocal string) error {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionProjectWrite); err != nil {
		return err
	}
	base, _, err := s.checkoutPath(ctx, actor, workspaceID, workRel, "", absLocal)
	if err != nil {
		return err
	}
	if filepath.Clean(base) == filepath.Clean(s.root) {
		return ErrOutsideCheckout
	}
	w, err := s.getForActor(ctx, actor, workspaceID)
	if err != nil {
		return err
	}
	wsDir := Dir(s.root, w.WorkspaceCode, w.RecID)
	if filepath.Clean(base) == filepath.Clean(wsDir) {
		return ErrOutsideCheckout
	}
	return utils.ClearDir(base, "")
}

func (s *Service) checkoutPath(ctx context.Context, actor iam.Actor, workspaceID, workRel, rel, absLocal string) (string, string, error) {
	w, err := s.getForActor(ctx, actor, workspaceID)
	if err != nil {
		return "", "", err
	}
	rel, err = utils.CleanRel(rel)
	if err != nil {
		return "", "", err
	}
	base := strings.TrimSpace(absLocal)
	if base != "" {
		base = filepath.Clean(base)
	} else {
		base = ProjectDir(s.root, w.WorkspaceCode, w.RecID, workRel)
		if !utils.UnderRoot(s.root, base) {
			return "", "", utils.ErrOutside
		}
	}
	if dest := utils.JoinUnder(base, rel); !utils.UnderRoot(base, dest) {
		return "", "", utils.ErrOutside
	}
	return base, rel, nil
}
