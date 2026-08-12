package zapret

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleGeneralBat = `@echo off
chcp 65001 > nul
cd /d "%~dp0"
call service.bat status_zapret
echo:

set "BIN=%~dp0bin\"
set "LISTS=%~dp0lists\"
cd /d %BIN%

start "zapret: %~n0" /min "%BIN%winws.exe" --wf-tcp=80,443,2053,2083,2087,2096,8443,%GameFilterTCP% --wf-udp=443,19294-19344,50000-50100,%GameFilterUDP% ^
--filter-udp=443 --hostlist="%LISTS%list-general.txt" --dpi-desync=fake --new ^
--filter-tcp=443 --hostlist="%LISTS%list-google.txt" --dpi-desync=multisplit
`

func TestParseWinwsArgsContent(t *testing.T) {
	gf := DisabledGameFilter()
	args, err := ParseWinwsArgsContent(sampleGeneralBat, `C:\Zapret`, gf)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--wf-tcp=80,443,2053,2083,2087,2096,8443,12") {
		t.Fatalf("tcp filter: %s", joined)
	}
	if !strings.Contains(joined, `--hostlist=C:\Zapret\lists\list-general.txt`) {
		t.Fatalf("hostlist: %s", joined)
	}
	if strings.Contains(joined, "%BIN%") || strings.Contains(joined, "^") {
		t.Fatalf("unexpanded: %s", joined)
	}
}

func TestShortName(t *testing.T) {
	if got := ShortName("general (ALT11)"); got != "ALT11" {
		t.Fatalf("got %q", got)
	}
	if got := ShortName("general"); got != "general" {
		t.Fatalf("base: %q", got)
	}
}

func TestNaturalSort(t *testing.T) {
	items := []Strategy{
		{Name: "general (ALT10)"},
		{Name: "general (ALT2)"},
		{Name: "general"},
		{Name: "general (ALT)"},
	}
	sortStrategies(items)
	got := []string{items[0].Name, items[1].Name, items[2].Name, items[3].Name}
	want := []string{"general", "general (ALT)", "general (ALT2)", "general (ALT10)"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sort: %v", got)
		}
	}
}

func TestDataRoot(t *testing.T) {
	root := DataRoot()
	if !strings.Contains(root, "Zapret Manager") {
		t.Fatalf("root %q", root)
	}
	if DefaultInstallDir() != filepath.Join(root, "zapret") {
		t.Fatal(DefaultInstallDir())
	}
}

func TestLocalVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.bat")
	if err := os.WriteFile(path, []byte("set \"LOCAL_VERSION=1.10.1\"\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LocalVersion(dir); got != "1.10.1" {
		t.Fatalf("got %q", got)
	}
}

func TestInstallZipUnwrapsPrefix(t *testing.T) {
	src := t.TempDir()
	root := filepath.Join(src, "zapret-discord-youtube-1.10.1")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "general.bat"), []byte("start winws.exe --ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "winws.exe"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "pack.zip")
	if err := zipDir(root, zipPath, "zapret-discord-youtube-1.10.1"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "Zapret")
	if err := InstallZip(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "general.bat")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "bin", "winws.exe")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "lists", "list-general-user.txt")); err != nil {
		t.Fatal(err)
	}
}
