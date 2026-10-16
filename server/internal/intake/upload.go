package intake

import (
	"context"
	"errors"
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

// pushProjectWars 上传一个工程目录里的全部 war。返回最后一份的远程路径和签名，供结果邮件使用。
func (s *Service) pushProjectWars(ctx context.Context, prj project.Project, n Notice) (local, remote, signed, until string, err error) {
	work, err := s.projectWork(ctx, prj)
	if err != nil {
		return "", "", "", "", err
	}
	wars, err := findWars(work)
	if err != nil {
		return "", "", "", "", err
	}
	for _, file := range wars {
		local, remote, signed, until, err = s.pushWarFile(ctx, prj, file, n)
		if err != nil {
			return local, remote, signed, until, err
		}
	}
	return local, remote, signed, until, nil
}

func (s *Service) pushWarFile(ctx context.Context, prj project.Project, local string, n Notice) (string, string, string, string, error) {
	var remote, signed, until string
	var err error
	name := filepath.Base(local)
	if root := strings.TrimSpace(n.OSSProjectRoot); root != "" {
		remote = remoteWar(root, name)
		key := objectKey(root, name)
		if s.upload != nil {
			err = s.upload(ctx, prj, local, remote)
		} else {
			err = s.putOSS(ctx, prj, local, key)
		}
		if err != nil {
			return local, remote, "", "", err
		}
		expire := s.ossExpire(ctx, prj)
		if s.sign != nil {
			signed, err = s.sign(key, expire)
		} else {
			signed, err = s.signOSS(ctx, prj, key, expire)
		}
		if err != nil {
			return local, remote, "", "", err
		}
		until = time.Now().Add(expire).In(receiptTZ).Format("2006-01-02 15:04:05")
	}
	if root := strings.TrimSpace(n.FTPProjectRoot); root != "" {
		dest := remoteWar(root, name)
		if remote == "" {
			remote = dest
		}
		if s.upload != nil {
			err = s.upload(ctx, prj, local, dest)
		} else {
			err = s.putFTP(ctx, prj, local, dest)
		}
		if err != nil {
			return local, remote, "", "", err
		}
	}
	if remote == "" {
		return local, "", "", "", errors.New("missing transfer path")
	}
	return local, remote, signed, until, nil
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
	return hostoss.SignGet(cfg, key, expire)
}

// ossConfig 读取已启用的 oss 环境，与环境页探测同一套 MapConfig。报文只提供对象路径。
func (s *Service) ossConfig(ctx context.Context, prj project.Project) (hostoss.Config, error) {
	if s.envs == nil {
		return hostoss.Config{}, errors.New("oss env is not configured")
	}
	cfg := hostoss.MapConfig(s.envs.KindConfig(ctx, prj.WorkspaceRecID, "oss"))
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return hostoss.Config{}, errors.New("oss env is not configured")
	}
	return cfg, nil
}

func (s *Service) ossExpire(ctx context.Context, prj project.Project) time.Duration {
	hours := 24
	if s.envs != nil {
		hours = hostoss.ExpireHours(s.envs.KindConfig(ctx, prj.WorkspaceRecID, "oss"))
	}
	return time.Duration(hours) * time.Hour
}

func (s *Service) putFTP(ctx context.Context, prj project.Project, local, remote string) error {
	if s.envs == nil {
		return errors.New("ftp env is not configured")
	}
	envID, kind := uploadEnv(project.ParseRecipe(prj.RecipeJSON))
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
