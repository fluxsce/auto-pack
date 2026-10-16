package serverupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"auto-pack/server/internal/config"
)

const (
	binBase    = "autopack"
	backupDir  = "server-update-backup"
	lockName   = "server-upgrade.lock"
	lockMaxAge = 15 * time.Minute
	// PackMaxBytes 发行包上传上限（512 MiB）。
	PackMaxBytes = 512 << 20
)

var packName = regexp.MustCompile(`^auto-pack-(\d+\.\d+\.\d+)-(linux|windows|darwin)-(amd64|arm64)\.(tar\.gz|tgz|zip)$`)

// Status 描述当前进程能否接受界面升级。
type Status struct {
	Version     string `json:"version"`
	GOOS        string `json:"goos"`
	GOARCH      string `json:"goarch"`
	Upgradable  bool   `json:"upgradable"`
	Message     string `json:"message,omitempty"`
	InstallRoot string `json:"install_root,omitempty"`
}

// Probe 检查本机是否处于可升级的安装目录。
func Probe() Status {
	st := Status{
		Version: config.AppVersion,
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
	}
	root, err := installRoot()
	if err != nil {
		st.Message = err.Error()
		return st
	}
	st.Upgradable = true
	st.InstallRoot = root
	st.Message = "可上传与本机系统、架构一致的发行包进行升级"
	return st
}

// Apply 校验发行包、合进安装目录并安排重启。成功后本进程会退出。
func Apply(archivePath, fileName, sum string) (string, error) {
	root, err := installRoot()
	if err != nil {
		return "", err
	}
	version, err := stage(root, archivePath, fileName, sum)
	if err != nil {
		return "", err
	}
	if err := schedule(root); err != nil {
		_ = restoreBackup(root)
		_ = os.Remove(stagedBinary(root))
		unlock(root)
		return "", err
	}
	go func() {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()
	return "已接收升级到 " + version + "，服务即将重启", nil
}

func stage(root, archivePath, fileName, sum string) (string, error) {
	meta, err := parsePackName(fileName)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(meta.goos, runtime.GOOS) || !strings.EqualFold(meta.goarch, runtime.GOARCH) {
		return "", fmt.Errorf("安装包是 %s/%s，本机是 %s/%s", meta.goos, meta.goarch, runtime.GOOS, runtime.GOARCH)
	}
	if !versionLess(config.AppVersion, meta.version) {
		return "", fmt.Errorf("安装包版本 %s 不高于当前 %s", meta.version, config.AppVersion)
	}
	if err := verifySHA(archivePath, sum); err != nil {
		return "", err
	}
	if err := lock(root); err != nil {
		return "", err
	}
	locked := true
	defer func() {
		if locked {
			unlock(root)
		}
	}()

	extract, err := os.MkdirTemp("", "ap-server-pack-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(extract)
	if err := extractArchive(archivePath, extract, fileName); err != nil {
		return "", err
	}
	payload, err := payloadRoot(extract)
	if err != nil {
		return "", err
	}
	if findBinary(payload, binBase) == "" {
		return "", errors.New("安装包里没有 bin/autopack")
	}
	if _, err := os.Stat(filepath.Join(payload, "web", "index.html")); err != nil {
		return "", errors.New("安装包里没有控制台页面 web/index.html")
	}
	if err := snapshot(root); err != nil {
		return "", err
	}
	if err := mergeTree(payload, root, binBase); err != nil {
		_ = restoreBackup(root)
		return "", err
	}
	newBin := findBinary(payload, binBase)
	if err := copyFile(newBin, stagedBinary(root)); err != nil {
		_ = restoreBackup(root)
		return "", err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(stagedBinary(root), 0o755)
	}
	locked = false
	return meta.version, nil
}

type packMeta struct {
	version string
	goos    string
	goarch  string
}

func parsePackName(fileName string) (packMeta, error) {
	base := filepath.Base(strings.TrimSpace(fileName))
	m := packName.FindStringSubmatch(base)
	if m == nil {
		return packMeta{}, errors.New("请上传发行包 auto-pack-版本-系统-架构.tar.gz 或 .zip")
	}
	return packMeta{version: m[1], goos: m[2], goarch: m[3]}, nil
}

func verifySHA(path, sum string) error {
	want := strings.ToLower(strings.TrimSpace(sum))
	if len(want) != 64 {
		return errors.New("请填写发行包的 SHA256（64 位十六进制）")
	}
	for _, r := range want {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return errors.New("请填写发行包的 SHA256（64 位十六进制）")
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != want {
		return errors.New("SHA256 与安装包不一致")
	}
	return nil
}

func versionLess(current, next string) bool {
	if current == "" || current == "dev" || current == "unknown" {
		return true
	}
	av, bv := versionParts(current), versionParts(next)
	n := len(av)
	if len(bv) > n {
		n = len(bv)
	}
	for i := 0; i < n; i++ {
		ai, bi := 0, 0
		if i < len(av) {
			ai = av[i]
		}
		if i < len(bv) {
			bi = bv[i]
		}
		if ai != bi {
			return ai < bi
		}
	}
	return false
}

func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	raw := strings.Split(v, ".")
	out := make([]int, 0, len(raw))
	for _, part := range raw {
		n := 0
		seen := false
		for _, r := range part {
			if r < '0' || r > '9' {
				break
			}
			seen = true
			n = n*10 + int(r-'0')
		}
		if !seen {
			out = append(out, 0)
			continue
		}
		out = append(out, n)
	}
	return out
}

func installRoot() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	binDir := filepath.Dir(exe)
	base := strings.TrimSuffix(filepath.Base(exe), ".exe")
	if !strings.EqualFold(base, binBase) {
		return "", errors.New("当前进程不是安装目录中的 autopack，不能在此升级（开发态请用发行包部署后再升级）")
	}
	if !strings.EqualFold(filepath.Base(binDir), "bin") {
		return "", errors.New("可执行文件须位于安装目录的 bin/ 下")
	}
	root := filepath.Dir(binDir)
	restart := "restart.sh"
	if runtime.GOOS == "windows" {
		restart = "restart.bat"
	}
	if _, err := os.Stat(filepath.Join(root, restart)); err != nil {
		return "", errors.New("当前不是用发行包 start/restart 脚本启动的安装目录，不能在此升级")
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		return "", err
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if filepath.Clean(wd) != filepath.Clean(rootReal) {
		return "", errors.New("请在安装目录中用 start/restart 脚本启动后再升级")
	}
	return root, nil
}

func binaryPath(root string) string {
	name := binBase
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(root, "bin", name)
}

func stagedBinary(root string) string {
	return binaryPath(root) + ".new"
}

func backupPath(root string) string {
	return filepath.Join(root, "data", backupDir)
}

func lockPath(root string) string {
	return filepath.Join(root, "data", lockName)
}

func lock(root string) error {
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		return err
	}
	path := lockPath(root)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		info, statErr := os.Stat(path)
		if statErr == nil && time.Since(info.ModTime()) > lockMaxAge {
			_ = os.Remove(path)
			file, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		}
	}
	if err != nil {
		if os.IsExist(err) {
			return errors.New("已有升级正在进行")
		}
		return err
	}
	_, _ = io.WriteString(file, strconv.Itoa(os.Getpid()))
	return file.Close()
}

func unlock(root string) {
	_ = os.Remove(lockPath(root))
}

func snapshot(root string) error {
	backup := backupPath(root)
	if err := os.RemoveAll(backup); err != nil {
		return err
	}
	if err := os.MkdirAll(backup, 0o755); err != nil {
		return err
	}
	items := []struct{ src, name string }{
		{filepath.Join(root, "web"), "web"},
		{filepath.Join(root, "script"), "script"},
	}
	for _, item := range items {
		info, err := os.Stat(item.src)
		if err != nil || !info.IsDir() {
			continue
		}
		if err := copyDir(item.src, filepath.Join(backup, item.name)); err != nil {
			return err
		}
	}
	return nil
}

func restoreBackup(root string) error {
	backup := backupPath(root)
	if info, err := os.Stat(filepath.Join(backup, "web")); err == nil && info.IsDir() {
		dest := filepath.Join(root, "web")
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
		if err := copyDir(filepath.Join(backup, "web"), dest); err != nil {
			return err
		}
	}
	if info, err := os.Stat(filepath.Join(backup, "script")); err == nil && info.IsDir() {
		dest := filepath.Join(root, "script")
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
		if err := copyDir(filepath.Join(backup, "script"), dest); err != nil {
			return err
		}
	}
	return nil
}
