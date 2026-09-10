package config

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const defaultConfigFile = "app.yaml"

// Config holds runtime configuration loaded from YAML.
type Config struct {
	Addr          string
	DBDriver      string
	DBPath        string
	DBDSN         string
	JWTSecret     string
	FrontendPath  string
	CryptKey      string
	GitCloneDepth int
	WorkspaceRoot string
	LogLevel      string
	LogFile       string
	ConfigDir     string
}

type fileConfig struct {
	Server struct {
		Addr string `yaml:"addr"`
	} `yaml:"server"`
	Database struct {
		Driver string `yaml:"driver"`
		Path   string `yaml:"path"`
		DSN    string `yaml:"dsn"`
	} `yaml:"database"`
	Web struct {
		JWTSecret string `yaml:"jwt_secret"`
		Frontend  struct {
			Path string `yaml:"path"`
		} `yaml:"frontend"`
	} `yaml:"web"`
	Crypt struct {
		Passphrase string `yaml:"passphrase"`
	} `yaml:"crypt"`
	Git struct {
		CloneDepth *int `yaml:"clone_depth"`
	} `yaml:"git"`
	Workspace struct {
		Root string `yaml:"root"`
	} `yaml:"workspace"`
	Log struct {
		Level string `yaml:"level"`
		File  string `yaml:"file"`
	} `yaml:"log"`
}

var configDirFlag string

func init() {
	flag.StringVar(&configDirFlag, "config", "", "config directory containing app.yaml")
}

// Load reads app.yaml from the config directory.
func Load() (Config, error) {
	if !flag.Parsed() {
		flag.Parse()
	}
	dir, err := resolveConfigDir()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, defaultConfigFile))
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	cfg := Config{
		Addr:          fc.Server.Addr,
		DBDriver:      fc.Database.Driver,
		DBPath:        fc.Database.Path,
		DBDSN:         fc.Database.DSN,
		JWTSecret:     fc.Web.JWTSecret,
		FrontendPath:  fc.Web.Frontend.Path,
		CryptKey:      fc.Crypt.Passphrase,
		WorkspaceRoot: fc.Workspace.Root,
		LogLevel:      fc.Log.Level,
		LogFile:       fc.Log.File,
		ConfigDir:     dir,
	}
	if fc.Git.CloneDepth != nil {
		cfg.GitCloneDepth = *fc.Git.CloneDepth
	} else {
		cfg.GitCloneDepth = 50
	}
	cfg.applyDefaults()
	cfg.resolvePaths()
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.DBDriver == "" {
		c.DBDriver = "sqlite"
	}
	if c.DBPath == "" {
		c.DBPath = "./data/db/autopack.db"
	}
	if c.JWTSecret == "" {
		c.JWTSecret = "auto-pack-dev-secret-change-in-production"
	}
	if c.CryptKey == "" {
		c.CryptKey = "auto-pack-dev-crypt-change-in-production"
	}
	if c.GitCloneDepth < 0 {
		c.GitCloneDepth = 50
	}
	if c.WorkspaceRoot == "" {
		c.WorkspaceRoot = "./data/workspaces"
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
}

func (c *Config) resolvePaths() {
	root := c.AppRoot()
	if !filepath.IsAbs(c.DBPath) {
		c.DBPath = filepath.Clean(filepath.Join(root, c.DBPath))
	}
	if c.FrontendPath != "" && !filepath.IsAbs(c.FrontendPath) {
		c.FrontendPath = filepath.Clean(filepath.Join(root, c.FrontendPath))
	}
	if !filepath.IsAbs(c.WorkspaceRoot) {
		c.WorkspaceRoot = filepath.Clean(filepath.Join(root, c.WorkspaceRoot))
	}
	if c.LogFile != "" && !filepath.IsAbs(c.LogFile) {
		c.LogFile = filepath.Clean(filepath.Join(root, c.LogFile))
	}
}

// AppRoot is the directory containing configs/.
func (c Config) AppRoot() string {
	root := filepath.Dir(c.ConfigDir)
	if !filepath.IsAbs(root) {
		if wd, err := os.Getwd(); err == nil {
			root = filepath.Join(wd, root)
		}
	}
	return filepath.Clean(root)
}

// DataDir is application data/ (database, job logs, workspace checkouts).
func (c Config) DataDir() string {
	return filepath.Join(c.AppRoot(), "data")
}

// SetupLog 把标准库 log 同时打到 stderr 和 log.file。file 为空则只打终端。
// 作业构建输出仍在 DataDir()/jobs/{id}/job.log，不走这里。
func (c Config) SetupLog() (*os.File, error) {
	if c.LogFile == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(c.LogFile), 0o755); err != nil {
		return nil, fmt.Errorf("log dir: %w", err)
	}
	f, err := os.OpenFile(c.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("log file: %w", err)
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	return f, nil
}

func resolveConfigDir() (string, error) {
	if configDirFlag != "" {
		return absDir(configDirFlag)
	}
	candidates := []string{"configs", "../configs", "server/configs"}
	if exe, err := os.Executable(); err == nil {
		candidates = append([]string{filepath.Join(filepath.Dir(exe), "configs")}, candidates...)
	}
	for _, c := range candidates {
		dir, err := absDir(c)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, defaultConfigFile)); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("config directory not found (need %s; use -config <dir>)", defaultConfigFile)
}

func absDir(path string) (string, error) {
	if !filepath.IsAbs(path) {
		if wd, err := os.Getwd(); err == nil {
			path = filepath.Join(wd, path)
		}
	}
	return filepath.Clean(path), nil
}
