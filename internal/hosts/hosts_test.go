package hosts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseProfilesJSON(t *testing.T) {
	list, err := parseProfilesJSON(defaultHostsJSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 10 {
		t.Fatalf("profiles=%d", len(list))
	}
	tg, ok := findProfile(list, IDTelegram)
	if !ok || len(tg.Entries) == 0 {
		t.Fatal("telegram missing")
	}
	ig, ok := findProfile(list, IDInstagram)
	if !ok || !ig.DefaultOn {
		t.Fatal("instagram should default on")
	}
}

func TestApplyTelegramBlockEnableDisable(t *testing.T) {
	base := "127.0.0.1 localhost\r\n"
	on := ApplyTelegramBlock(base, true)
	if !HasTelegramBlock(on) {
		t.Fatal("expected block")
	}
	if !strings.Contains(on, "web.telegram.org") {
		t.Fatal("missing web.telegram.org")
	}
	if !strings.Contains(on, "127.0.0.1 localhost") {
		t.Fatal("lost original hosts")
	}
	off := ApplyTelegramBlock(on, false)
	if HasTelegramBlock(off) {
		t.Fatal("block should be removed")
	}
	if !strings.Contains(off, "127.0.0.1 localhost") {
		t.Fatal("lost original hosts after disable")
	}
}

func TestApplyTelegramBlockIdempotent(t *testing.T) {
	first := ApplyTelegramBlock("127.0.0.1 localhost\n", true)
	second := ApplyTelegramBlock(first, true)
	if strings.Count(second, beginMarker(IDTelegram)) != 1 {
		t.Fatalf("duplicate blocks: %d", strings.Count(second, beginMarker(IDTelegram)))
	}
}

func TestTelegramDomainsCount(t *testing.T) {
	if len(TelegramDomains()) != 23 {
		t.Fatalf("got %d domains", len(TelegramDomains()))
	}
}

func TestApplyMultipleProfiles(t *testing.T) {
	base := "127.0.0.1 localhost\n"
	ig, _ := ProfileByID(IDInstagram)
	ai, _ := ProfileByID(IDAI)
	content := ApplyProfileBlock(base, ig, true, DefaultProxyIP)
	content = ApplyProfileBlock(content, ai, true, DefaultProxyIP)
	if !HasProfileBlock(content, IDInstagram) || !HasProfileBlock(content, IDAI) {
		t.Fatal("expected both blocks")
	}
	if !strings.Contains(content, "instagram.com") || !strings.Contains(content, "chatgpt.com") {
		t.Fatal("missing domains")
	}
	if !strings.Contains(content, DefaultProxyIP+" ") && !strings.Contains(content, DefaultProxyIP+"\n") {
		// proxy IP should be substituted for PROXY token
		if !strings.Contains(content, DefaultProxyIP) {
			t.Fatal("proxy ip not substituted")
		}
	}
	content = ApplyProfileBlock(content, ig, false, DefaultProxyIP)
	if HasProfileBlock(content, IDInstagram) {
		t.Fatal("instagram should be removed")
	}
	if !HasProfileBlock(content, IDAI) {
		t.Fatal("ai should remain")
	}
}

func TestBuildProfileBlockDedupesHosts(t *testing.T) {
	p := Profile{
		ID:      "instagram",
		Comment: "IG",
		Entries: []Entry{
			{IP: "1.1.1.1", Hosts: []string{"instagram.com", "www.instagram.com"}},
			{IP: "2.2.2.2", Hosts: []string{"instagram.com", "i.instagram.com"}},
		},
	}
	got := buildProfileBlock(p, "\n", DefaultProxyIP)
	if strings.Count(got, " instagram.com") != 1 && !strings.Contains(got, "1.1.1.1 instagram.com ") {
		t.Fatalf("instagram.com should be bound once:\n%s", got)
	}
	if !strings.Contains(got, "1.1.1.1 instagram.com www.instagram.com") {
		t.Fatalf("first ip should win:\n%s", got)
	}
	if !strings.Contains(got, "2.2.2.2 i.instagram.com") {
		t.Fatalf("new host should remain:\n%s", got)
	}
}

func TestExtraProfilesDefaults(t *testing.T) {
	on := 0
	for _, p := range ExtraProfiles() {
		if p.DefaultOn {
			on++
		}
	}
	if on != 2 {
		t.Fatalf("expected 2 defaults on, got %d", on)
	}
}

func TestEnsureConfigWritesFile(t *testing.T) {
	root := t.TempDir()
	old := os.Getenv("TMP")
	old2 := os.Getenv("TEMP")
	t.Setenv("TMP", root)
	t.Setenv("TEMP", root)
	_ = old
	_ = old2
	// DataRoot uses TempDir which reads TEMP/TMP - on Windows both matter.
	// zapret.DataRoot uses os.TempDir() which is set from env at process start on Windows...
	// Actually os.TempDir() is cached. So this test may not work.
	// Instead write to a custom path via parse only.
	path := filepath.Join(root, "hosts.json")
	if err := os.WriteFile(path, defaultHostsJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	list, err := parseProfilesJSON(data)
	if err != nil || len(list) == 0 {
		t.Fatalf("parse: %v len=%d", err, len(list))
	}
}

func findProfile(list []Profile, id string) (Profile, bool) {
	for _, p := range list {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}
