package intake

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	hostftp "auto-pack/server/internal/host/ftp"
	hostoss "auto-pack/server/internal/host/oss"
	hostssh "auto-pack/server/internal/host/ssh"
	"auto-pack/server/internal/project"
)

var (
	errTaskDir = errors.New("invalid intake work dir")
	errNoWar   = errors.New("war not found")
)

// pushProjectWars 上传工程产物。
// 报文写了 resourceFiles 时只推这些文件。OSS 且 resourceFiles 空白时不过滤：工程目录下每个文件原样上传，每个目录先打成同名 zip。
// 两个及以上 sce 开头的前端目录（含 sce-vdatahub）打成一个包，只有一个时压缩该目录本身。
// FTP 未写 resourceFiles 时只上传目录里的 war。
// 多份文件的远程路径和临时地址用换行拼在一起，供回执和邮件使用。
func (s *Service) pushProjectWars(ctx context.Context, prj project.Project, n Notice) (local, remote, signed, until, backup string, err error) {
	work, err := s.projectWork(ctx, prj)
	if err != nil {
		return "", "", "", "", "", err
	}
	var files []string
	var cleanup func()
	if strings.TrimSpace(n.OSSProjectRoot) != "" && len(cleanResourceNames(n.ResourceFiles)) == 0 {
		files, cleanup, err = collectOSSPushFiles(work)
	} else {
		files, err = selectPushFiles(work, n.ResourceFiles)
	}
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		if errors.Is(err, errNoWar) {
			return "", "", "", "", "", fmt.Errorf("%w：%s", errNoWar, work)
		}
		return "", "", "", "", "", err
	}
	at := time.Now()
	var remotes, urls, backups []string
	for _, file := range files {
		var oneBackup string
		local, remote, signed, until, oneBackup, err = s.pushWarFile(ctx, prj, file, n, at)
		if err != nil {
			if remote != "" {
				remotes = append(remotes, remote)
			}
			if oneBackup != "" {
				backups = append(backups, oneBackup)
			}
			return local, strings.Join(remotes, "\n"), strings.Join(urls, "\n"), until, strings.Join(backups, "；"), err
		}
		if remote != "" {
			remotes = append(remotes, remote)
		}
		if signed != "" {
			urls = append(urls, signed)
		}
		if oneBackup != "" {
			backups = append(backups, oneBackup)
		}
	}
	return local, strings.Join(remotes, "\n"), strings.Join(urls, "\n"), until, strings.Join(backups, "；"), nil
}

// pushWarFile 按报文路径只走一条通道。有 ossProjectRoot 只推 OSS，只有 ftpProjectRoot 才推 FTP。
func (s *Service) pushWarFile(ctx context.Context, prj project.Project, local string, n Notice, at time.Time) (string, string, string, string, string, error) {
	name := filepath.Base(local)
	if root := strings.TrimSpace(n.OSSProjectRoot); root != "" {
		remote := remoteWar(root, name)
		key := objectKey(root, name)
		backup, err := s.backupOSS(ctx, prj, key, at)
		if err != nil {
			return local, remote, "", "", "", labelPushErr("OSS", err)
		}
		if s.upload != nil {
			err = s.upload(ctx, prj, local, remote)
		} else {
			err = s.putOSS(ctx, prj, local, key)
		}
		if err != nil {
			return local, remote, "", "", backup, labelPushErr("OSS", err)
		}
		expire := s.ossExpire(ctx, prj)
		var signed string
		if s.sign != nil {
			signed, err = s.sign(key, expire)
		} else {
			signed, err = s.signOSS(ctx, prj, key, expire)
		}
		if err != nil {
			return local, remote, "", "", backup, labelPushErr("OSS", err)
		}
		until := time.Now().Add(expire).In(receiptTZ).Format("2006-01-02 15:04:05")
		return local, remote, signed, until, backup, nil
	}
	if root := strings.TrimSpace(n.FTPProjectRoot); root != "" {
		dest := remoteWar(root, name)
		var err error
		if s.upload != nil {
			err = s.upload(ctx, prj, local, dest)
		} else {
			err = s.putFTP(ctx, prj, local, dest)
		}
		if err != nil {
			return local, dest, "", "", "", labelPushErr("FTP", err)
		}
		return local, dest, "", "", "", nil
	}
	return local, "", "", "", "", errors.New("missing transfer path")
}

func labelPushErr(via string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s %w", via, err)
}

// backupOSS 在覆盖前把已有对象复制到传入 OSS 路径下的 备份/yyyyMMdd/HHmmss/。时间用东八区。对象不存在时返回空路径。
func (s *Service) backupOSS(ctx context.Context, prj project.Project, key string, at time.Time) (string, error) {
	if s == nil || s.upload != nil {
		return "", nil
	}
	cfg, err := s.ossConfig(ctx, prj)
	if err != nil {
		return "", err
	}
	exists, err := hostoss.Exists(cfg, key)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}
	dest := ossBackupKey(key, at)
	if err := hostoss.Copy(cfg, key, dest); err != nil {
		return "", err
	}
	return "/" + strings.TrimLeft(dest, "/"), nil
}

func ossBackupKey(objectKey string, at time.Time) string {
	objectKey = strings.Trim(strings.ReplaceAll(objectKey, "\\", "/"), "/")
	dir := path.Dir(objectKey)
	name := path.Base(objectKey)
	if dir == "." {
		dir = ""
	}
	if at.IsZero() {
		at = time.Now()
	}
	at = at.In(receiptTZ)
	folder := "备份/" + at.Format("20060102") + "/" + at.Format("150405")
	if dir != "" {
		folder = dir + "/" + folder
	}
	return folder + "/" + name
}

// collectOSSPushFiles 收集工程目录的直接子项。文件原样加入，目录按自动推送的规则压缩。
func collectOSSPushFiles(work string) (files []string, cleanup func(), err error) {
	rels, err := listProjectEntries(work)
	if err != nil {
		return nil, func() {}, err
	}
	return project.CollectPushPaths(work, rels)
}

// listProjectEntries 列出工程目录下要推送的名称，跳过点目录和点文件。
func listProjectEntries(work string) ([]string, error) {
	work = strings.TrimSpace(work)
	st, err := os.Stat(work)
	if err != nil || !st.IsDir() {
		return nil, errNoWar
	}
	entries, err := os.ReadDir(work)
	if err != nil {
		return nil, err
	}
	rels := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || name == "." || name == ".." || strings.HasPrefix(name, ".") {
			continue
		}
		rels = append(rels, name)
	}
	if len(rels) == 0 {
		return nil, errNoWar
	}
	sort.Strings(rels)
	return rels, nil
}

func findWars(work string) ([]string, error) {
	work = strings.TrimSpace(work)
	st, err := os.Stat(work)
	if err != nil || !st.IsDir() {
		return nil, errNoWar
	}
	var wars []string
	err = filepath.WalkDir(work, func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if name != work && (d.Name() == ".git" || strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".war") {
			wars = append(wars, name)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(wars) == 0 {
		return nil, errNoWar
	}
	sort.Strings(wars)
	return wars, nil
}

// selectPushFiles 按资源文件名挑选要推送的文件。names 为空时退回全部 war。
func selectPushFiles(work string, names []string) ([]string, error) {
	wanted := cleanResourceNames(names)
	if len(wanted) == 0 {
		return findWars(work)
	}
	index, err := indexWorkFiles(work)
	if err != nil {
		return nil, err
	}
	var missing []string
	out := make([]string, 0, len(wanted))
	seen := map[string]struct{}{}
	for _, name := range wanted {
		full, ok := matchWorkFile(index, name)
		if !ok {
			missing = append(missing, name)
			continue
		}
		if _, dup := seen[full]; dup {
			continue
		}
		seen[full] = struct{}{}
		out = append(out, full)
	}
	if len(missing) > 0 {
		return nil, errors.New("未找到资源文件 " + strings.Join(missing, ", "))
	}
	if len(out) == 0 {
		return nil, errNoWar
	}
	return out, nil
}

func cleanResourceNames(names []string) []string {
	out := make([]string, 0, len(names))
	seen := map[string]struct{}{}
	for _, name := range names {
		name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
		name = strings.Trim(name, "/")
		if name == "" || name == "." {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	return out
}

type workFile struct {
	full string
	rel  string
}

func indexWorkFiles(work string) ([]workFile, error) {
	work = strings.TrimSpace(work)
	st, err := os.Stat(work)
	if err != nil || !st.IsDir() {
		return nil, errNoWar
	}
	var files []workFile
	err = filepath.WalkDir(work, func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if name != work && (d.Name() == ".git" || strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(work, name)
		if err != nil {
			return err
		}
		files = append(files, workFile{full: name, rel: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	return files, nil
}

func matchWorkFile(files []workFile, token string) (string, bool) {
	token = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(token, "\\", "/")))
	token = strings.Trim(token, "/")
	if token == "" {
		return "", false
	}
	base := path.Base(token)
	var baseHit string
	for _, file := range files {
		rel := strings.ToLower(file.rel)
		if rel == token || strings.HasSuffix(rel, "/"+token) {
			return file.full, true
		}
		if baseHit == "" && strings.ToLower(path.Base(file.rel)) == base {
			baseHit = file.full
		}
	}
	if baseHit != "" {
		return baseHit, true
	}
	return "", false
}

func (s *Service) putOSS(ctx context.Context, prj project.Project, local, key string) error {
	cfg, err := s.ossConfig(ctx, prj)
	if err != nil {
		return err
	}
	return hostoss.PutFile(cfg, local, key)
}

func (s *Service) signOSS(ctx context.Context, prj project.Project, key string, expire time.Duration) (string, error) {
	cfg, err := s.ossConfig(ctx, prj)
	if err != nil {
		return "", err
	}
	return hostoss.SignGetMode(cfg, key, expire, "attachment")
}

// ossConfig 优先用该 mq 环境关联的 OSS。未关联时用已启用的 oss 环境。报文只提供对象路径。
func (s *Service) ossConfig(ctx context.Context, prj project.Project) (hostoss.Config, error) {
	if s.envs == nil {
		return hostoss.Config{}, errors.New("oss env is not configured")
	}
	if raw, ok, err := s.linkedOSSRaw(ctx); err != nil {
		return hostoss.Config{}, err
	} else if ok {
		cfg := hostoss.MapConfig(raw)
		if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
			return hostoss.Config{}, errors.New("linked oss env is not configured")
		}
		return cfg, nil
	}
	cfg := hostoss.MapConfig(s.envs.KindConfig(ctx, prj.WorkspaceRecID, "oss"))
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return hostoss.Config{}, errors.New("oss env is not configured")
	}
	return cfg, nil
}

func (s *Service) ossExpire(ctx context.Context, prj project.Project) time.Duration {
	hours := 24
	if raw, ok, err := s.linkedOSSRaw(ctx); err == nil && ok {
		hours = hostoss.ExpireHours(raw)
	} else if s.envs != nil {
		hours = hostoss.ExpireHours(s.envs.KindConfig(ctx, prj.WorkspaceRecID, "oss"))
	}
	return time.Duration(hours) * time.Hour
}

func (s *Service) putFTP(ctx context.Context, prj project.Project, local, remote string) error {
	if s.envs == nil {
		return errors.New("ftp env is not configured")
	}
	envID, kind := "", ""
	if id := configString(s.mqConfig(ctx), "ftp_env_rec_id"); id != "" {
		envID = id
	} else {
		envID, kind = uploadEnv(project.ParseRecipe(prj.RecipeJSON))
	}
	if envID == "" {
		return errors.New("ftp env is not configured")
	}
	e, raw, err := s.envs.DecryptRow(ctx, envID)
	if err != nil {
		return err
	}
	if k := strings.ToLower(strings.TrimSpace(e.EnvKind)); k != "" {
		kind = k
	}
	switch kind {
	case "sftp", "ssh":
		cfg := hostssh.MapConfig(raw)
		cfg.RemotePath = ""
		sess, err := hostssh.DialSession(cfg)
		if err != nil {
			return err
		}
		defer sess.Close()
		return sess.UploadFile(local, remote, nil)
	default:
		cfg := hostftp.MapConfig(raw)
		cfg.RemoteDir = ""
		sess, err := hostftp.Dial(cfg)
		if err != nil {
			return err
		}
		defer sess.Close()
		return sess.UploadFile(local, remote, nil)
	}
}

func uploadEnv(recipe project.Recipe) (envID, kind string) {
	if id := strings.TrimSpace(recipe.FTPEnvRecID); id != "" {
		return id, "ftp"
	}
	for _, act := range recipe.Actions() {
		if !project.ActionEnabled(act) {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(act.Kind))
		if k == "ftp" || k == "sftp" || k == "ssh" {
			return strings.TrimSpace(act.EnvRecID), k
		}
	}
	return "", ""
}

func objectKey(root, name string) string {
	return strings.TrimLeft(remoteWar(root, name), "/")
}

func remoteWar(root, name string) string {
	root = strings.TrimSpace(strings.ReplaceAll(root, "\\", "/"))
	if root == "" {
		return name
	}
	if !strings.HasSuffix(root, "/") {
		root += "/"
	}
	return path.Clean(root + name)
}
