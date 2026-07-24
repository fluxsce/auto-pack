package shell

import (
	"bytes"
	"io"
	"sync"

	"auto-pack/backend/pkg/redact"
)

type syncLineSink struct {
	w  io.Writer
	mu sync.Mutex
}

func (s *syncLineSink) writeLine(line []byte) error {
	line = bytes.TrimRight(line, "\r")
	text := redact.Redact(DecodeProcessOutput(line))
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := io.WriteString(s.w, text+"\n"); err != nil {
		return err
	}
	if syncer, ok := s.w.(interface{ Sync() error }); ok {
		_ = syncer.Sync()
	}
	return nil
}

// liveLogWriter line-buffers one stream (stdout or stderr) before writing to the shared sink.
type liveLogWriter struct {
	sink *syncLineSink
	buf  []byte
}

func newLiveLogWriter(sink *syncLineSink) *liveLogWriter {
	return &liveLogWriter{sink: sink}
}

func (l *liveLogWriter) Write(p []byte) (int, error) {
	l.buf = append(l.buf, p...)
	for {
		i := bytes.IndexByte(l.buf, '\n')
		if i < 0 {
			break
		}
		line := l.buf[:i]
		l.buf = l.buf[i+1:]
		if err := l.sink.writeLine(line); err != nil {
			return len(p), err
		}
	}
	return len(p), nil
}

func (l *liveLogWriter) Flush() error {
	if len(l.buf) == 0 {
		return nil
	}
	line := l.buf
	l.buf = nil
	return l.sink.writeLine(line)
}
