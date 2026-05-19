package logx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Logger struct {
	verbose bool
	path    string
	max     int

	mu    sync.Mutex
	lines []string
}

func New(verbose bool, path string, keepLines int) *Logger {
	if keepLines < 10 {
		keepLines = 10
	}
	if strings.TrimSpace(path) != "" {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
	}
	return &Logger{
		verbose: verbose,
		path:    path,
		max:     keepLines,
	}
}

func (l *Logger) Debug(format string, args ...any) {
	l.write("DEBUG", format, args...)
}

func (l *Logger) Info(format string, args ...any) {
	l.write("INFO", format, args...)
}

func (l *Logger) Warn(format string, args ...any) {
	l.write("WARN", format, args...)
}

func (l *Logger) Error(format string, args ...any) {
	l.write("ERROR", format, args...)
}

func (l *Logger) write(level, format string, args ...any) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s [%s] %s", time.Now().Format("2006-01-02T15:04:05"), level, msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.lines = append(l.lines, line)
	if len(l.lines) > l.max {
		l.lines = append([]string(nil), l.lines[len(l.lines)-l.max:]...)
	}
	if l.path != "" {
		_ = os.WriteFile(l.path, []byte(strings.Join(l.lines, "\n")), 0o644)
	}
	if l.verbose || level == "ERROR" || level == "WARN" {
		fmt.Println(line)
	}
}
