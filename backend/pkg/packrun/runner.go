package packrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/maven"
	"auto-pack/backend/pkg/mavendeps"
	"auto-pack/backend/pkg/nodejs"
	"auto-pack/backend/pkg/projectcfg"

	"github.com/google/uuid"
)

// Runner executes project builds asynchronously.
type Runner struct {
	projects     *repository.ProjectRepository
	workspaces   *repository.WorkspaceRepository
	env          *repository.EnvRepository
	builds       *repository.BuildRepository
	dataDir      string
	scheduler    *Scheduler
	projectLocks projectBuildLocker
	syncCoord    workspaceSourceSyncCoordinator

	mu sync.Mutex
}

func NewRunner(
	projects *repository.ProjectRepository,
	workspaces *repository.WorkspaceRepository,
	env *repository.EnvRepository,
	builds *repository.BuildRepository,
	dataDir string,
) *Runner {
	r := &Runner{
		projects:   projects,
		workspaces: workspaces,
		env:        env,
		builds:     builds,
		dataDir:    dataDir,
	}
	r.scheduler = NewScheduler(r, projects)
	return r
}

// Scheduler returns the cron scheduler (started via StartScheduler).
func (r *Runner) Scheduler() *Scheduler {
	return r.scheduler
}

// StartScheduler loads cron jobs from the database and begins scheduling.
func (r *Runner) StartScheduler() error {
	return r.scheduler.Start()
}

// Trigger starts a new build for the project.
func (r *Runner) Trigger(projectRecID string, mode TriggerMode) (*repository.PackBuild, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	project, err := r.projects.FindByRecID(projectRecID)
	if err != nil {
		return nil, err
	}
	if project.IsEnabled != 1 {
		return nil, fmt.Errorf("project is disabled")
	}
	cfg, err := projectcfg.ParseConfigJSON(project.ConfigJSON)
	if err != nil {
		return nil, err
	}
	if mode != TriggerManual {
		if err := r.checkBuildDependencies(project, cfg); err != nil {
			return nil, err
		}
	}
	running, err := r.builds.FindRunningByProject(projectRecID)
	if err != nil {
		return nil, err
	}
	if running != nil {
		return nil, repository.ErrBuildRunning
	}
	releaseProject, ok := r.tryAcquireProjectBuild(projectRecID)
	if !ok {
		return nil, repository.ErrBuildRunning
	}

	num, err := r.builds.NextBuildNum(projectRecID)
	if err != nil {
		return nil, err
	}

	logDir := filepath.Join(r.dataDir, "builds")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	recID := uuid.New().String()
	logPath := filepath.Join(logDir, recID+".log")

	now := time.Now().UTC()
	build := &repository.PackBuild{
		RecID:        recID,
		ProjectRecID: projectRecID,
		BuildNum:     num,
		Status:       repository.BuildStatusPending,
		Message:      "queued (" + mode.String() + ")",
		LogPath:      logPath,
		StartedAt:    &now,
	}
	if err := r.builds.Insert(build); err != nil {
		releaseProject()
		return nil, err
	}
	// Release immediately: executeBuild re-acquires the lock only for source sync
	// and the main build phase. Holding it across Maven upstream installs caused
	// circular deadlocks when pipeline triggered interdependent projects in parallel.
	releaseProject()

	go r.executeBuild(build.RecID, mode)

	created, err := r.builds.FindByRecID(recID)
	if err != nil {
		return build, nil
	}
	return created, nil
}

func embedHTTPSAuth(rawURL string, cfg envtypes.GitConfig) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if strings.TrimSpace(cfg.AuthToken) != "" {
		u.User = url.UserPassword("oauth2", cfg.AuthToken)
	} else if u2 := strings.TrimSpace(cfg.AuthUsername); u2 != "" {
		u.User = url.UserPassword(u2, cfg.AuthPassword)
	}
	return u.String()
}

func (r *Runner) buildShellEnv(project *repository.PackProject) ([]string, error) {
	env := append([]string{}, os.Environ()...)
	appendEnv := func(recID string) error {
		if recID == "" {
			return nil
		}
		item, err := r.env.FindByRecID(recID)
		if err != nil {
			return err
		}
		switch item.EnvType {
		case envtypes.TypeJDK:
			var cfg envtypes.JDKConfig
			if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
				return err
			}
			if home := strings.TrimSpace(cfg.JavaHome); home != "" {
				env = append(env, "JAVA_HOME="+home)
				env = prependPath(env, filepath.Join(home, "bin"))
			}
		case envtypes.TypeMaven:
			var cfg envtypes.MavenConfig
			if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
				return err
			}
			if home := strings.TrimSpace(cfg.MavenHome); home != "" {
				env = append(env, "MAVEN_HOME="+home)
				env = prependPath(env, filepath.Join(home, "bin"))
			}
			if home := strings.TrimSpace(cfg.JavaHome); home != "" {
				env = append(env, "JAVA_HOME="+home)
				env = prependPath(env, filepath.Join(home, "bin"))
			}
		case envtypes.TypeNodeJS:
			var cfg envtypes.NodeJSConfig
			if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
				return err
			}
			if p := strings.TrimSpace(cfg.NodePath); p != "" {
				if resolved, err := nodejs.ResolveBinPath(p, "node"); err == nil {
					env = prependPath(env, filepath.Dir(resolved))
				}
			}
			if p := strings.TrimSpace(cfg.NpmPath); p != "" {
				if resolved, err := nodejs.ResolveBinPath(p, "npm"); err == nil {
					env = prependPath(env, filepath.Dir(resolved))
				}
			}
		}
		return nil
	}
	if err := appendEnv(project.JdkEnvRecID); err != nil {
		return nil, err
	}
	if err := appendEnv(project.MavenEnvRecID); err != nil {
		return nil, err
	}
	if err := appendEnv(project.NodejsEnvRecID); err != nil {
		return nil, err
	}
	return appendUTF8ConsoleEnv(env), nil
}

func (r *Runner) mavenSettingsPath(project *repository.PackProject) string {
	if project == nil || strings.TrimSpace(project.MavenEnvRecID) == "" {
		return ""
	}
	item, err := r.env.FindByRecID(project.MavenEnvRecID)
	if err != nil {
		return ""
	}
	var cfg envtypes.MavenConfig
	if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
		return ""
	}
	return maven.ResolveSettingsPath(cfg.SettingsXML)
}

func (r *Runner) mavenHome(project *repository.PackProject) string {
	if project == nil || strings.TrimSpace(project.MavenEnvRecID) == "" {
		return ""
	}
	item, err := r.env.FindByRecID(project.MavenEnvRecID)
	if err != nil {
		return ""
	}
	var cfg envtypes.MavenConfig
	if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.MavenHome)
}

func (r *Runner) mavenLocalRepository(project *repository.PackProject) string {
	return mavendeps.ResolveLocalRepository(r.mavenSettingsPath(project), r.mavenHome(project))
}

const javaUTF8Flags = "-Dfile.encoding=UTF-8 -Dsun.stdout.encoding=UTF-8 -Dsun.stderr.encoding=UTF-8"

func appendUTF8ConsoleEnv(env []string) []string {
	if runtime.GOOS != "windows" {
		return env
	}
	env = mergeToolOption(env, "JAVA_TOOL_OPTIONS", javaUTF8Flags)
	env = mergeToolOption(env, "_JAVA_OPTIONS", javaUTF8Flags)
	env = mergeToolOption(env, "MAVEN_OPTS", javaUTF8Flags)
	if !hasEnvKey(env, "PYTHONIOENCODING") {
		env = append(env, "PYTHONIOENCODING=utf-8")
	}
	return env
}

func mergeToolOption(env []string, key, option string) []string {
	prefix := key + "="
	for i, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			continue
		}
		current := strings.TrimSpace(entry[len(prefix):])
		if current == "" {
			env[i] = prefix + option
			return env
		}
		if strings.Contains(current, option) {
			return env
		}
		env[i] = prefix + current + " " + option
		return env
	}
	return append(env, prefix+option)
}

func hasEnvKey(env []string, key string) bool {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}

func prependPath(env []string, dir string) []string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return env
	}
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			sep := string(os.PathListSeparator)
			env[i] = "PATH=" + dir + sep + strings.TrimPrefix(e, "PATH=")
			return env
		}
	}
	return append(env, "PATH="+dir)
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// ReadLogTail reads log file from offset up to maxBytes.
func ReadLogTail(logPath string, offset int64, maxBytes int) (content string, total int64, err error) {
	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}
	if maxBytes > 1024*1024 {
		maxBytes = 1024 * 1024
	}
	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, nil
		}
		return "", 0, err
	}
	total = info.Size()
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	f, err := os.Open(logPath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	if _, err := f.Seek(offset, 0); err != nil {
		return "", total, err
	}
	buf := make([]byte, maxBytes)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", total, err
	}
	return string(buf[:n]), total, nil
}
