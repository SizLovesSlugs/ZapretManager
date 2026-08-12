package app

import (
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
