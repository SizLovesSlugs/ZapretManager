package zapret

import (
	"path/filepath"
	"strings"
)

// GameStrategy is injected into the main winws arg list as an extra --new block.
type GameStrategy struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	UDPPorts string `json:"udpPorts"`
	FakeUDP  string `json:"-"` // filename under bin/
}

var builtinGameStrategies = []GameStrategy{
	{
		ID:       "siz-loves-dbd-1",
		Name:     "Siz Loves DbD v1",
		UDPPorts: "7771-8000,61456,61457",
		FakeUDP:  "quic_initial_www_google_com.bin",
	},
}

func GameStrategies() []GameStrategy {
	return builtinGameStrategies
}

func DefaultGameStrategy() GameStrategy {
	return builtinGameStrategies[0]
}

func GameStrategyByID(id string) (GameStrategy, bool) {
	id = strings.TrimSpace(id)
	for _, g := range builtinGameStrategies {
		if strings.EqualFold(g.ID, id) || strings.EqualFold(g.Name, id) {
			return g, true
		}
	}
	return GameStrategy{}, false
}

func ResolveGameStrategy(id string) GameStrategy {
	if g, ok := GameStrategyByID(id); ok {
		return g
	}
	return DefaultGameStrategy()
}

// InjectGameStrategy merges UDP ports into --wf-udp and appends a Windows-adapted
// game --new block. OpenWRT hopbyhop6 is dropped (unsupported in winws).
func InjectGameStrategy(args []string, root string, g GameStrategy) []string {
	if g.UDPPorts == "" {
		return args
	}
	out := append([]string(nil), args...)
	for i, a := range out {
		if strings.HasPrefix(a, "--wf-udp=") {
			out[i] = mergeFlagCSV(a, g.UDPPorts)
		}
	}
	fake := g.FakeUDP
	if fake == "" {
		fake = "quic_initial_www_google_com.bin"
	}
	fakePath := filepath.Join(BinDir(root), fake)
	out = append(out,
		"--new",
		"--filter-udp="+g.UDPPorts,
		"--dpi-desync=fake",
		"--dpi-desync-autottl=2",
		"--dpi-desync-repeats=6",
		"--dpi-desync-any-protocol=1",
		"--dpi-desync-fake-unknown-udp="+fakePath,
		"--dpi-desync-cutoff=n2",
	)
	return out
}

func mergeFlagCSV(flagWithValue, extraCSV string) string {
	eq := strings.IndexByte(flagWithValue, '=')
	if eq < 0 {
		return flagWithValue
	}
	prefix := flagWithValue[:eq+1]
	cur := flagWithValue[eq+1:]
	seen := map[string]bool{}
	var parts []string
	for _, p := range strings.Split(cur, ",") {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		parts = append(parts, p)
	}
	for _, p := range strings.Split(extraCSV, ",") {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		parts = append(parts, p)
	}
	return prefix + strings.Join(parts, ",")
}
