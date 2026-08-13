package app

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"zapret-manager/internal/zapret"
)

const (
	logTimeLayout = "2006-01-02 15:04:05"
	logTailBytes  = 512 * 1024
	logMaxEntries = 300
)

type LogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

type LogsView struct {
	Path    string     `json:"path"`
	Entries []LogEntry `json:"entries"`
}

type fileLogger struct {
	mu   sync.Mutex
	path string
}

func newFileLogger() *fileLogger {
	dir := zapret.LogsDir()
	_ = os.MkdirAll(dir, 0o755)
	return &fileLogger{path: zapret.LogFile()}
}

func (l *fileLogger) Error(msg string) {
	l.write("ERROR", msg)
}

func (l *fileLogger) Warn(msg string) {
	l.write("WARN", msg)
}

func (l *fileLogger) write(level, msg string) {
	if l == nil {
		return
	}
	msg = strings.TrimSpace(strings.ReplaceAll(msg, "\n", " "))
	if msg == "" {
		return
	}
	line := formatLogLine(time.Now(), level, msg)
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = os.MkdirAll(zapret.LogsDir(), 0o755)
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(line)
	_ = f.Close()
}

func (l *fileLogger) Clear() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	dir := filepath.Dir(l.path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(l.path)
	if err != nil {
		return err
	}
	return f.Close()
}

func (l *fileLogger) View() LogsView {
	view := LogsView{Path: zapret.LogFile(), Entries: []LogEntry{}}
	if l == nil {
		return view
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	data, err := readLogTail(l.path, logTailBytes)
	if err != nil {
		return view
	}
	view.Entries = parseLogEntries(data, logMaxEntries)
	return view
}

func formatLogLine(t time.Time, level, msg string) string {
	if level == "" {
		level = "ERROR"
	}
	return t.Format(logTimeLayout) + " " + level + " " + msg + "\n"
}

func parseLogLine(line string) (LogEntry, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return LogEntry{}, false
	}
	for _, prefix := range []string{" ERROR ", " WARN ", " INFO "} {
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		stamp := strings.TrimSpace(line[:idx])
		if _, err := time.Parse(logTimeLayout, stamp); err != nil {
			return LogEntry{}, false
		}
		msg := strings.TrimSpace(line[idx+len(prefix):])
		if msg == "" {
			return LogEntry{}, false
		}
		return LogEntry{Time: stamp, Message: msg}, true
	}
	return LogEntry{}, false
}

func parseLogEntries(data []byte, limit int) []LogEntry {
	if limit <= 0 {
		limit = logMaxEntries
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var all []LogEntry
	for sc.Scan() {
		entry, ok := parseLogLine(sc.Text())
		if !ok {
			continue
		}
		all = append(all, entry)
	}
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all
}

func readLogTail(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size == 0 {
		return nil, nil
	}
	if size > maxBytes {
		if _, err := f.Seek(size-maxBytes, io.SeekStart); err != nil {
			return nil, err
		}
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if size > maxBytes {
		if i := bytes.IndexByte(data, '\n'); i >= 0 && i+1 < len(data) {
			data = data[i+1:]
		}
	}
	return data, nil
}
