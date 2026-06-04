package logger

// DefaultMaxSizeMB is the per-file size threshold before rotation.
const DefaultMaxSizeMB = 100

// DefaultMaxBackups is how many rotated log files to retain.
const DefaultMaxBackups = 10

// Options configures the global logger.
type Options struct {
	// Level is one of debug, info, warn, error (default info).
	Level string
	// File is the log file path; empty means stderr only.
	File string
	// MaxSizeMB is the max size in megabytes of a single log file before rotation.
	MaxSizeMB int
	// MaxBackups is the number of rotated log files to keep.
	MaxBackups int
	// MaxAgeDays removes files older than this many days (0 = no age limit).
	MaxAgeDays int
	// AlsoStdout mirrors logs to stderr when File is set.
	AlsoStdout bool
}

// DefaultOptions returns rotation defaults: 100MB per file, 10 backups.
func DefaultOptions() Options {
	return Options{
		Level:      "info",
		MaxSizeMB:  DefaultMaxSizeMB,
		MaxBackups: DefaultMaxBackups,
		AlsoStdout: true,
	}
}

func (o Options) withDefaults() Options {
	d := DefaultOptions()
	if o.Level == "" {
		o.Level = d.Level
	}
	if o.MaxSizeMB <= 0 {
		o.MaxSizeMB = d.MaxSizeMB
	}
	if o.MaxBackups <= 0 {
		o.MaxBackups = d.MaxBackups
	}
	if o.File == "" {
		o.AlsoStdout = true
	}
	return o
}

// FromConfig builds Options from application log settings.
func FromConfig(level, file string, maxSizeMB, maxBackups, maxAgeDays int, alsoStdout bool) Options {
	return Options{
		Level:      level,
		File:       file,
		MaxSizeMB:  maxSizeMB,
		MaxBackups: maxBackups,
		MaxAgeDays: maxAgeDays,
		AlsoStdout: alsoStdout,
	}
}
