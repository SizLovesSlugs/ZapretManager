package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLogLine(t *testing.T) {
	line := formatLogLine(time.Date(2026, 8, 12, 20, 23, 15, 0, time.Local), `Не удалось получить релизы: Get "https://github.com/Flowseal/zapret-discord-youtube/releases": EOF`)
	entry, ok := parseLogLine(line)
	if !ok {
		t.Fatal("expected parsed entry")
	}
	if entry.Time != "2026-08-12 20:23:15" {
		t.Fatalf("time: %s", entry.Time)
	}
	if !strings.Contains(entry.Message, "EOF") {
		t.Fatalf("message: %s", entry.Message)
	}
}

func TestParseLogEntriesNewestFirst(t *testing.T) {
	data := []byte(strings.Join([]string{
		"2026-08-12 20:00:00 ERROR first",
		"not a log",
		"2026-08-12 20:01:00 ERROR second",
	}, "\n"))
	got := parseLogEntries(data, 10)
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Message != "second" || got[1].Message != "first" {
		t.Fatalf("%+v", got)
	}
}

func TestClearLogs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("2026-08-12 20:00:00 ERROR keep-me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &fileLogger{path: path}
	if err := l.Clear(); err != nil {
		t.Fatal(err)
	}
	view := l.View()
	if len(view.Entries) != 0 {
		t.Fatalf("%+v", view.Entries)
	}
}
