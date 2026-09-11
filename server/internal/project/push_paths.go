package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/server/internal/host/utils"
)

const (
	sceMergeZip     = "sce.zip"
	sceVdatahubZip  = "sce-vdatahub.zip"
	sceVdatahubRoot = "sce-vdatahub"
	sceStaticPrefix = "sce"
)

// PushSource 是一次推送收集的工程根 + 相对路径。
type PushSource struct {
	Work string
	Rel  string
}

// IsSceStaticDirName 判断目录名是否为前端静态资源树（sce 开头）。
func IsSceStaticDirName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, ".zip")
	return strings.HasPrefix(n, sceStaticPrefix)
}

func isVdatahubStaticDirName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, ".zip")
	return n == sceVdatahubRoot || strings.HasPrefix(n, sceVdatahubRoot+"-") || strings.HasPrefix(n, sceVdatahubRoot+"_")
}

// SceMergeZipName 多个 sce 目录合并后的 zip 文件名。
// 全部是 sce-vdatahub 及其变体时用 sce-vdatahub.zip，否则用 sce.zip。
func SceMergeZipName(names []string) string {
	if len(names) == 0 {
		return sceMergeZip
	}
	for _, n := range names {
		if !isVdatahubStaticDirName(n) {
			return sceMergeZip
		}
	}
	return sceVdatahubZip
}

// IsSceMergeZipName 判断是否为 sce 前端目录合并包文件名。
func IsSceMergeZipName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return n == sceMergeZip || n == sceVdatahubZip
}

// CollectPushPaths 收集工程检出目录内用户勾选的相对路径。
// 文件原样加入；目录打成 {目录名}.zip（包内以目录名为根，跳过 .git / 点目录）。
// 两个及以上 sce 开头的目录合并成一个 zip，解压后仍是原来的目录名。
// 项可为精确相对路径或通配符（* ? **）。禁止工程根，避免整仓误传。cleanup 删除本次产生的临时 zip。
func CollectPushPaths(work string, rels []string) (files []string, cleanup func(), err error) {
	if strings.TrimSpace(work) == "" {
		return nil, func() {}, fmt.Errorf("work dir required")
	}
	srcs := make([]PushSource, 0, len(rels))
	for _, rel := range rels {
		srcs = append(srcs, PushSource{Work: work, Rel: rel})
	}
	return CollectPushSources(srcs)
}

// CollectPushSources 从多个工程根收集推送文件，规则与 CollectPushPaths 相同。
func CollectPushSources(srcs []PushSource) (files []string, cleanup func(), err error) {
	cleanup = func() {}
	if len(srcs) == 0 {
		return nil, cleanup, ErrNoArtifacts
	}
	var tmp string
	cleanup = func() {
		if tmp != "" {
			_ = os.RemoveAll(tmp)
		}
	}
	items, resolveErr := resolvePushItems(srcs)
	if resolveErr != nil {
		cleanup()
		return nil, func() {}, resolveErr
	}
	var sceAbs, sceNames []string
	for _, it := range items {
		if it.sce {
			sceAbs = append(sceAbs, it.dest)
			sceNames = append(sceNames, it.base)
		}
	}
	mergeSce := len(sceAbs) >= 2
	ensureTmp := func() error {
		if tmp != "" {
			return nil
		}
		var mkErr error
		tmp, mkErr = os.MkdirTemp("", "ap-ftp-push-")
		return mkErr
	}
	var out []string
	seenOut := map[string]struct{}{}
	addOut := func(path string) error {
		if _, ok := seenOut[path]; ok {
			return nil
		}
		if len(out) >= maxPushFiles {
			return fmt.Errorf("too many files to push (>%d)", maxPushFiles)
		}
		seenOut[path] = struct{}{}
		out = append(out, path)
		return nil
	}
	merged := false
	for _, it := range items {
		if it.sce && mergeSce {
			if merged {
				continue
			}
			if mkErr := ensureTmp(); mkErr != nil {
				cleanup()
				return nil, func() {}, mkErr
			}
			zipPath := filepath.Join(tmp, SceMergeZipName(sceNames))
			if zipErr := utils.ZipSources(sceAbs, zipPath, skipHiddenPushDir); zipErr != nil {
				cleanup()
				return nil, func() {}, zipErr
			}
			if addErr := addOut(zipPath); addErr != nil {
				cleanup()
				return nil, func() {}, addErr
			}
			merged = true
			continue
		}
		if !it.dir {
			if addErr := addOut(it.dest); addErr != nil {
				cleanup()
				return nil, func() {}, addErr
			}
			continue
		}
		if mkErr := ensureTmp(); mkErr != nil {
			cleanup()
			return nil, func() {}, mkErr
		}
		zipPath := filepath.Join(tmp, it.base+".zip")
		if zipErr := utils.ZipDirAs(it.dest, zipPath, it.base, skipHiddenPushDir); zipErr != nil {
			cleanup()
			return nil, func() {}, zipErr
		}
		if addErr := addOut(zipPath); addErr != nil {
			cleanup()
			return nil, func() {}, addErr
		}
	}
	if len(out) == 0 {
		cleanup()
		return nil, func() {}, ErrNoArtifacts
	}
	return out, cleanup, nil
}

type resolvedPushItem struct {
	dest string
	base string
	dir  bool
	sce  bool
}

func resolvePushItems(srcs []PushSource) ([]resolvedPushItem, error) {
	var items []resolvedPushItem
	seenAbs := map[string]struct{}{}
	for _, src := range srcs {
		if strings.TrimSpace(src.Work) == "" {
			return nil, fmt.Errorf("work dir required")
		}
		expanded, err := expandPushRels(src.Work, []string{src.Rel})
		if err != nil {
			return nil, err
		}
		for _, rel := range expanded {
			dest, rel, resolveErr := utils.Resolve(src.Work, rel)
			if resolveErr != nil {
				return nil, resolveErr
			}
			if rel == "" {
				return nil, fmt.Errorf("%w: project root", ErrNoArtifacts)
			}
			if _, ok := seenAbs[dest]; ok {
				continue
			}
			seenAbs[dest] = struct{}{}
			st, statErr := os.Stat(dest)
			if statErr != nil {
				return nil, statErr
			}
			base := filepath.Base(dest)
			dir := st.IsDir()
			items = append(items, resolvedPushItem{
				dest: dest,
				base: base,
				dir:  dir,
				sce:  dir && IsSceStaticDirName(base),
			})
		}
	}
	return items, nil
}
