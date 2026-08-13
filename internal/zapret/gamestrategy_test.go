package zapret

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectGameStrategy(t *testing.T) {
	root := `C:\Zapret`
	args := []string{
		"--wf-tcp=80,443",
		"--wf-udp=443,50000-50100",
		"--filter-tcp=443",
		"--dpi-desync=fake",
	}
	g := DefaultGameStrategy()
	out := InjectGameStrategy(args, root, g)
	if !strings.Contains(out[1], "7771-8000") || !strings.Contains(out[1], "61456") {
		t.Fatalf("wf-udp not merged: %s", out[1])
	}
	joined := strings.Join(out, " ")
	if !strings.Contains(joined, "--filter-udp="+g.UDPPorts) {
		t.Fatalf("missing filter-udp: %s", joined)
	}
	if strings.Contains(joined, "hopbyhop6") {
		t.Fatal("hopbyhop6 must not be used on winws")
	}
	fake := filepath.Join(BinDir(root), g.FakeUDP)
	if !strings.Contains(joined, fake) {
		t.Fatalf("missing fake udp path %s in %s", fake, joined)
	}
	if strings.Count(joined, "--new") < 1 {
		t.Fatal("expected --new block")
	}
}

func TestInjectGameStrategiesMultiple(t *testing.T) {
	root := `C:\Zapret`
	args := []string{"--wf-udp=443"}
	games := GameStrategies()
	if len(games) < 2 {
		t.Fatal("expected at least two builtin game strategies")
	}
	out := InjectGameStrategies(args, root, games)
	joined := strings.Join(out, " ")
	if strings.Count(joined, "--new") < 2 {
		t.Fatalf("expected a --new block per strategy: %s", joined)
	}
	if !strings.Contains(out[0], "7771-8000") || !strings.Contains(out[0], "7000-9000") {
		t.Fatalf("wf-udp missing both port sets: %s", out[0])
	}
	if !strings.Contains(joined, "--filter-udp="+games[0].UDPPorts) {
		t.Fatalf("missing dbd filter: %s", joined)
	}
	if !strings.Contains(joined, "--filter-udp="+games[1].UDPPorts) {
		t.Fatalf("missing rocket league filter: %s", joined)
	}
}

func TestResolveEnabledDefaults(t *testing.T) {
	got := ResolveEnabled(nil)
	if len(got) != len(GameStrategies()) {
		t.Fatalf("defaults: %d", len(got))
	}
	got = ResolveEnabled(map[string]bool{got[0].ID: false})
	if len(got) != len(GameStrategies())-1 {
		t.Fatalf("one off: %d", len(got))
	}
}

func TestMergeFlagCSVIdempotent(t *testing.T) {
	got := mergeFlagCSV("--wf-udp=443,7771-8000", "7771-8000,61456")
	if got != "--wf-udp=443,7771-8000,61456" {
		t.Fatalf("got %s", got)
	}
}
