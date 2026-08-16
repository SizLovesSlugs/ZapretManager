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
	if !strings.Contains(joined, "--filter-udp="+compactPortCSV(g.UDPPorts)) {
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
	var games []GameStrategy
	for _, g := range GameStrategies() {
		if g.DefaultOn {
			games = append(games, g)
		}
	}
	if len(games) < 2 {
		t.Fatal("expected at least two default-on game strategies")
	}
	out := InjectGameStrategies(args, root, games)
	joined := strings.Join(out, " ")
	if strings.Count(joined, "--new") != 1 {
		t.Fatalf("same desync should merge into one --new: %s", joined)
	}
	wf := strings.TrimPrefix(out[0], "--wf-udp=")
	if strings.Contains(wf, "3400-61457") || strings.Contains(wf, "1024-65535") {
		t.Fatalf("wf-udp must not span game-port gaps: %s", wf)
	}
	for _, port := range []int{443, 7777, 8500, 61456} {
		if !portCSVCovers(wf, port) {
			t.Fatalf("wf-udp %s does not cover %d", wf, port)
		}
	}
	var filter string
	for _, a := range out {
		if strings.HasPrefix(a, "--filter-udp=") {
			filter = strings.TrimPrefix(a, "--filter-udp=")
			break
		}
	}
	for _, port := range []int{3400, 7777, 8500, 12500, 27000, 61457} {
		if !portCSVCovers(filter, port) {
			t.Fatalf("filter-udp %s does not cover %d", filter, port)
		}
	}
	if strings.Contains(filter, "7700-8100") {
		t.Fatalf("overlapping 7700-8100 should be absorbed: %s", filter)
	}
	if !strings.Contains(joined, "--dpi-desync-autottl=6") || !strings.Contains(joined, "--dpi-desync-cutoff=n4") {
		t.Fatalf("merged profile should match DbD: %s", joined)
	}
}

func TestResolveEnabledDefaults(t *testing.T) {
	got := ResolveEnabled(nil)
	want := 0
	for _, g := range GameStrategies() {
		if g.DefaultOn {
			want++
		}
	}
	if len(got) != want {
		t.Fatalf("defaults: %d want %d", len(got), want)
	}
	got = ResolveEnabled(map[string]bool{got[0].ID: false})
	if len(got) != want-1 {
		t.Fatalf("one off: %d", len(got))
	}
}

func TestInjectDbDv1Profile(t *testing.T) {
	v1, ok := GameStrategyByID("siz-loves-dbd-1")
	if !ok {
		t.Fatal("missing v1")
	}
	if _, ok := GameStrategyByID("siz-loves-dbd-2"); ok {
		t.Fatal("v2 must be removed")
	}
	out := InjectGameStrategy([]string{"--wf-udp=443"}, `C:\Zapret`, v1)
	joined := strings.Join(out, " ")
	if !strings.Contains(joined, "quic_initial_www_google_com.bin") {
		t.Fatalf("missing v1 fake: %s", joined)
	}
	if strings.Contains(joined, "dbankcloud") {
		t.Fatal("v1 must keep the google QUIC fake")
	}
	if !strings.Contains(joined, "--dpi-desync-autottl=6") {
		t.Fatalf("missing autottl=6: %s", joined)
	}
	if !strings.Contains(joined, "--dpi-desync-repeats=6") {
		t.Fatalf("missing repeats=6: %s", joined)
	}
	if !strings.Contains(joined, "--dpi-desync-cutoff=n4") {
		t.Fatalf("missing cutoff=n4: %s", joined)
	}
	if strings.Contains(joined, "udplen") || strings.Contains(joined, "ipfrag") || strings.Contains(joined, "hopbyhop") {
		t.Fatalf("unexpected desync extras: %s", joined)
	}
}

func TestCompactPortCSV(t *testing.T) {
	got := compactPortCSV("7700-8100,7000-9000,3400-3500,7771-8000,61456,61457")
	want := "3400-3500,7000-9000,61456-61457"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestMergeUDPPortCSVFitsBudget(t *testing.T) {
	existing := "12,443,19294-19344,50000-50100"
	extra := "7700-8100,7000-9000,3400-3500,4300-4400,27000-27100,12000-13000,7771-8000,61456,61457"
	got := mergeUDPPortCSV(existing, extra, maxUDPFilterFragment)
	if estimateUDPFilterFragment(parsePortCSV(got)) > maxUDPFilterFragment {
		t.Fatalf("filter too long: %s (%d)", got, estimateUDPFilterFragment(parsePortCSV(got)))
	}
	if strings.Contains(got, "3400-61457") || strings.Contains(got, "1024-65535") {
		t.Fatalf("must not span gaps: %s", got)
	}
	if portCSVCovers(got, 12) {
		t.Fatalf("dummy GameFilter port 12 leaked into wf-udp: %s", got)
	}
	for _, port := range []int{443, 8000, 19300, 50050, 61456} {
		if !portCSVCovers(got, port) {
			t.Fatalf("%s does not cover %d", got, port)
		}
	}
}

func TestMergeUDPAlreadyCovered(t *testing.T) {
	got := mergeUDPPortCSV("443,1024-65535", "7000-9000,61456", maxUDPFilterFragment)
	if got != "443,1024-65535" {
		t.Fatalf("got %s", got)
	}
}
