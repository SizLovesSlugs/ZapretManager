package github

import (
	"strings"
	"testing"
)

func TestParseExeDownloadURLs(t *testing.T) {
	page := `
		<a href="/SizLovesSlugs/ZapretManager/releases/download/0.2/Zapret%20Manager%200.2.exe">exe</a>
		<a href="/SizLovesSlugs/ZapretManager/releases/download/0.2/notes.txt">txt</a>
	`
	got := parseExeDownloadURLs(page, "https://github.com", ManagerRepo)
	if len(got) != 1 || !strings.Contains(got[0], "Zapret%20Manager%200.2.exe") {
		t.Fatalf("%v", got)
	}
}

func TestConventionalExeURLs(t *testing.T) {
	got := conventionalExeURLs("https://github.com", ManagerRepo, "1.0-Beta")
	want := "https://github.com/SizLovesSlugs/ZapretManager/releases/download/1.0-Beta/Zapret%20Manager%201.0%20Beta.exe"
	if len(got) < 1 || got[0] != want {
		t.Fatalf("%v", got)
	}
	foundDotted := false
	for _, u := range got {
		if strings.Contains(u, "Zapret.Manager.1.0.Beta.exe") {
			foundDotted = true
			break
		}
	}
	if !foundDotted {
		t.Fatalf("missing github-sanitized name: %v", got)
	}
}
