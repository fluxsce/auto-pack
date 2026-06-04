package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const defaultConfigFile = "app.yaml"

// AppVersion is the console release version (update in code when shipping).
const AppVersion = "1.0.0"

// Config holds application configuration loaded from YAML.
type Config struct {
	Addr         string
	DBPath       string
	JWTSecret    string
	FrontendPath string
	Log          LogConfig
	ConfigDir    string
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level      string
	File       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	AlsoStdout bool
}

type fileConfig struct {
	Server struct {
		Addr string `yaml:"addr"`
	} `yaml:"server"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	Web struct {
		JWTSecret string `yaml:"jwt_secret"`
		Frontend  struct {
			Path string `yaml:"path"`
		} `yaml:"frontend"`
	} `yaml:"web"`
	Log struct {
		Level      string `yaml:"level"`
		File       string `yaml:"file"`
		MaxSizeMB  int    `yaml:"max_size_mb"`
		MaxBackups int    `yaml:"max_backups"`
		MaxAgeDays int    `yaml:"max_age_days"`
		AlsoStdout *bool  `yaml:"also_stdout"`
	} `yaml:"log"`
}

var configDirFlag string

func init() {
	flag.StringVar(&configDirFlag, "config", "", "config directory containing app.yaml")
}

// Load reads app.yaml from the config directory (see resolveConfigDir).
func Load() (Config, error) {
	if !flag.Parsed() {
		flag.Parse()
	}

	dir, err := resolveConfigDir()
	if err != nil {
		return Config{}, err
	}

	path := filepath.Join(dir, defaultConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	logStdout := true
	if fc.Log.AlsoStdout != nil {
		logStdout = *fc.Log.AlsoStdout
	}

	cfg := Config{
		Addr:         fc.Server.Addr,
		DBPath:       fc.Database.Path,
		JWTSecret:    fc.Web.JWTSecret,
		FrontendPath: fc.Web.Frontend.Path,
		Log: LogConfig{
			Level:      fc.Log.Level,
			File:       fc.Log.File,
			MaxSizeMB:  fc.Log.MaxSizeMB,
			MaxBackups: fc.Log.MaxBackups,
			MaxAgeDays: fc.Log.MaxAgeDays,
			AlsoStdout: logStdout,
		},
		ConfigDir: dir,
	}
	cfg.applyDefaults()
	cfg.resolvePaths()

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.DBPath == "" {
		c.DBPath = "./data/db/autopack.db"
	}
	if c.JWTSecret == "" {
		c.JWTSecret = "auto-pack-dev-secret-change-in-production"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.File == "" {
		c.Log.File = "./logs/autopack.log"
	}
	if c.Log.MaxSizeMB <= 0 {
		c.Log.MaxSizeMB = 100
	}
	if c.Log.MaxBackups <= 0 {
		c.Log.MaxBackups = 10
	}
}

// resolvePaths makes relative paths absolute against the application root (parent of configs/).
func (c *Config) resolvePaths() {
	root := filepath.Dir(c.ConfigDir)
	if !filepath.IsAbs(root) {
		if wd, err := os.Getwd(); err == nil {
			root = filepath.Join(wd, root)
		}
	}
	root = filepath.Clean(root)

	if !filepath.IsAbs(c.DBPath) {
		c.DBPath = filepath.Clean(filepath.Join(root, c.DBPath))
	}
	if c.FrontendPath != "" && !filepath.IsAbs(c.FrontendPath) {
		c.FrontendPath = filepath.Clean(filepath.Join(root, c.FrontendPath))
	}
	if c.Log.File != "" && !filepath.IsAbs(c.Log.File) {
		c.Log.File = filepath.Clean(filepath.Join(root, c.Log.File))
	}
}

// AppRoot is the directory containing configs/ (application root).
func (c Config) AppRoot() string {
	root := filepath.Dir(c.ConfigDir)
	if !filepath.IsAbs(root) {
		if wd, err := os.Getwd(); err == nil {
			root = filepath.Join(wd, root)
		}
	}
	return filepath.Clean(root)
}

// DataDir is application data/ (database, build logs). Per-workspace project roots
// default to DataDir/workspaces/{workspace_code} when root_dir is unset.
func (c Config) DataDir() string {
	return filepath.Join(c.AppRoot(), "data")
}

func (c *Config) validate() error {
	if c.JWTSecret == "auto-pack-dev-secret-change-in-production" {
		// allow in dev; production operators should change yaml
	}
	return nil
}

func resolveConfigDir() (string, error) {
	if configDirFlag != "" {
		return absDir(configDirFlag)
	}

	candidates := []string{
		"configs",
		"../configs",
		"backend/configs",
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append([]string{
			filepath.Join(filepath.Dir(exe), "configs"),
		}, candidates...)
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
