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
	got := conventionalExeURLs("https://github.com", ManagerRepo, "0.2")
	want := "https://github.com/SizLovesSlugs/ZapretManager/releases/download/0.2/Zapret%20Manager%200.2.exe"
	if len(got) < 1 || got[0] != want {
		t.Fatalf("%v", got)
	}
}
