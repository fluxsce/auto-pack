package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	mu       sync.RWMutex
	instance *slog.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	rotator  io.Closer
)

// Init configures the process-wide logger. Safe to call once at startup.
func Init(opts Options) error {
	opts = opts.withDefaults()

	level, err := parseLevel(opts.Level)
	if err != nil {
		return err
	}

	var w io.Writer = os.Stderr
	var nextRotator io.Closer

	if opts.File != "" {
		if err := os.MkdirAll(dirOf(opts.File), 0o755); err != nil {
			return fmt.Errorf("create log dir: %w", err)
		}
		fileW := &lumberjack.Logger{
			Filename:   opts.File,
			MaxSize:    opts.MaxSizeMB,
			MaxBackups: opts.MaxBackups,
			MaxAge:     opts.MaxAgeDays,
			LocalTime:  true,
		}
		nextRotator = fileW
		if opts.AlsoStdout {
			w = io.MultiWriter(os.Stderr, fileW)
		} else {
			w = fileW
		}
	}

	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level, AddSource: level == slog.LevelDebug})

	mu.Lock()
	if rotator != nil {
		_ = rotator.Close()
	}
	rotator = nextRotator
	instance = slog.New(h)
	mu.Unlock()
	return nil
}

// Close releases the rotating log file handle, if any.
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if rotator == nil {
		return nil
	}
	err := rotator.Close()
	rotator = nil
	return err
}

// L returns the global slog.Logger.
func L() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return instance
}

func Debug(msg string, args ...any) { L().Debug(msg, args...) }
func Info(msg string, args ...any)  { L().Info(msg, args...) }
func Warn(msg string, args ...any)  { L().Warn(msg, args...) }
func Error(msg string, args ...any) { L().Error(msg, args...) }

// Fatal logs at error level and exits with code 1.
func Fatal(msg string, args ...any) {
	L().Error(msg, args...)
	os.Exit(1)
}

func parseLevel(s string) (slog.Leveler, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return nil, fmt.Errorf("unknown log level %q", s)
	}
}

func dirOf(path string) string {
	i := strings.LastIndexAny(path, `/\`)
	if i < 0 {
		return "."
	}
	return path[:i]
}
