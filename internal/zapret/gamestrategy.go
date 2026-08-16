package zapret

import (
	"path/filepath"
	"strconv"
	"strings"
)

const defaultFakeUDP = "quic_initial_www_google_com.bin"

// GameStrategy is injected into the main winws arg list as an extra --new block.
type GameStrategy struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	UDPPorts  string `json:"udpPorts"`
	DefaultOn bool   `json:"defaultOn"`
	FakeUDP   string `json:"-"` // filename under bin/
	Desync    string `json:"-"`
	AutoTTL   string `json:"-"`
	TTL       string `json:"-"`
	Repeats   int    `json:"-"`
	Cutoff    string `json:"-"`
	UDPLenInc int    `json:"-"`
	IPFragPos int    `json:"-"`
}

var builtinGameStrategies = []GameStrategy{
	{
		ID:        "siz-loves-dbd-1",
		Name:      "Siz Loves DbD v1",
		UDPPorts:  "7771-8000,61456,61457",
		DefaultOn: true,
		FakeUDP:   defaultFakeUDP,
		Desync:    "fake",
		AutoTTL:   "2",
		Repeats:   6,
		Cutoff:    "n2",
	},
	{
		// winws accepts one phase-1 mode plus one phase-2 mode.
		// fake+udplen+ipfrag2 is invalid and winws exits on start.
		// fake,ipfrag2 mutates the original UDP datagram so a TSPU
		// that ignores a plain QUIC fake still has to reassemble fragments.
		ID:        "siz-loves-dbd-2",
		Name:      "Siz Loves DbD v2",
		UDPPorts:  "7771-8000,61456,61457",
		DefaultOn: false,
		FakeUDP:   defaultFakeUDP,
		Desync:    "fake,ipfrag2",
		TTL:       "1",
		Repeats:   10,
		Cutoff:    "n4",
		IPFragPos: 8,
	},
	{
		ID:        "siz-loves-rocket-league-1",
		Name:      "Siz Loves Rocket League v1",
		UDPPorts:  "7700-8100,7000-9000,3400-3500,4300-4400,27000-27100,12000-13000",
		DefaultOn: true,
		FakeUDP:   defaultFakeUDP,
		Desync:    "fake",
		AutoTTL:   "2",
		Repeats:   6,
		Cutoff:    "n2",
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

// ResolveEnabled returns builtin strategies that are on in enabled.
// Missing keys fall back to DefaultOn.
func ResolveEnabled(enabled map[string]bool) []GameStrategy {
	var out []GameStrategy
	for _, g := range builtinGameStrategies {
		on, ok := enabled[g.ID]
		if !ok {
			on = g.DefaultOn
		}
		if on {
			out = append(out, g)
		}
	}
	return dropSupersededGames(out)
}

func dropSupersededGames(games []GameStrategy) []GameStrategy {
	hasV2 := false
	for _, g := range games {
		if g.ID == "siz-loves-dbd-2" {
			hasV2 = true
			break
		}
	}
	if !hasV2 {
		return games
	}
	out := games[:0]
	for _, g := range games {
		if g.ID == "siz-loves-dbd-1" {
			continue
		}
		out = append(out, g)
	}
	return out
}

func InjectGameStrategies(args []string, root string, games []GameStrategy) []string {
	groups := mergeGameGroups(games)
	if len(groups) == 0 {
		return args
	}
	out := mergeWfUDP(args, joinGameUDP(groups))
	for _, g := range groups {
		out = appendGameDesync(out, root, g)
	}
	return out
}

// InjectGameStrategy merges UDP ports into --wf-udp and appends a Windows-adapted
// game --new block. OpenWRT hopbyhop6 is dropped (unsupported in winws).
func InjectGameStrategy(args []string, root string, g GameStrategy) []string {
	return InjectGameStrategies(args, root, []GameStrategy{g})
}

func (g GameStrategy) normalized() GameStrategy {
	if g.FakeUDP == "" {
		g.FakeUDP = defaultFakeUDP
	}
	if g.Desync == "" {
		g.Desync = "fake"
	}
	if g.Repeats <= 0 {
		g.Repeats = 6
	}
	if g.Cutoff == "" {
		g.Cutoff = "n2"
	}
	if g.AutoTTL == "" && g.TTL == "" {
		g.AutoTTL = "2"
	}
	return g
}

func (g GameStrategy) profileKey() string {
	g = g.normalized()
	return strings.Join([]string{
		g.FakeUDP,
		g.Desync,
		g.AutoTTL,
		g.TTL,
		strconv.Itoa(g.Repeats),
		g.Cutoff,
		strconv.Itoa(g.UDPLenInc),
		strconv.Itoa(g.IPFragPos),
	}, "|")
}

func mergeGameGroups(games []GameStrategy) []GameStrategy {
	type acc struct {
		g     GameStrategy
		ports []string
	}
	order := make([]string, 0, len(games))
	byProfile := map[string]*acc{}
	for _, g := range games {
		ports := compactPortCSV(g.UDPPorts)
		if ports == "" {
			continue
		}
		g = g.normalized()
		key := g.profileKey()
		if cur, ok := byProfile[key]; ok {
			cur.ports = append(cur.ports, ports)
			continue
		}
		byProfile[key] = &acc{g: g, ports: []string{ports}}
		order = append(order, key)
	}
	out := make([]GameStrategy, 0, len(order))
	for _, key := range order {
		cur := byProfile[key]
		cur.g.UDPPorts = compactPortCSV(strings.Join(cur.ports, ","))
		out = append(out, cur.g)
	}
	return out
}

func appendGameDesync(args []string, root string, g GameStrategy) []string {
	g = g.normalized()
	fakePath := filepath.Join(BinDir(root), g.FakeUDP)
	out := append(args,
		"--new",
		"--filter-udp="+g.UDPPorts,
		"--dpi-desync="+g.Desync,
	)
	if g.TTL != "" {
		out = append(out, "--dpi-desync-ttl="+g.TTL)
	}
	if g.AutoTTL != "" {
		out = append(out, "--dpi-desync-autottl="+g.AutoTTL)
	}
	out = append(out,
		"--dpi-desync-repeats="+strconv.Itoa(g.Repeats),
		"--dpi-desync-any-protocol=1",
	)
	if g.UDPLenInc > 0 {
		out = append(out, "--dpi-desync-udplen-increment="+strconv.Itoa(g.UDPLenInc))
	}
	if g.IPFragPos > 0 {
		out = append(out, "--dpi-desync-ipfrag-pos-udp="+strconv.Itoa(g.IPFragPos))
	}
	return append(out,
		"--dpi-desync-fake-unknown-udp="+fakePath,
		"--dpi-desync-cutoff="+g.Cutoff,
	)
}
