package zapret

import (
	"sort"
	"strconv"
	"strings"
)

type portRange struct {
	from, to int
}

// Older Flowseal/winws builds a WinDivert UDP fragment in a 256-byte buffer.
// Too many --wf-udp ranges overflow it, winws exits, and the service "starts then dies".
const maxUDPFilterFragment = 240

func parsePortCSV(csv string) []portRange {
	var out []portRange
	for _, p := range strings.Split(csv, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		from, to, ok := parsePortToken(p)
		if !ok {
			continue
		}
		out = append(out, portRange{from, to})
	}
	return compactRanges(out)
}

func parsePortToken(p string) (int, int, bool) {
	a, b, found := strings.Cut(p, "-")
	from, err := strconv.Atoi(strings.TrimSpace(a))
	if err != nil || from < 1 || from > 65535 {
		return 0, 0, false
	}
	if !found {
		return from, from, true
	}
	to, err := strconv.Atoi(strings.TrimSpace(b))
	if err != nil || to < 1 || to > 65535 {
		return 0, 0, false
	}
	if to < from {
		from, to = to, from
	}
	return from, to, true
}

func compactRanges(in []portRange) []portRange {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool {
		if in[i].from != in[j].from {
			return in[i].from < in[j].from
		}
		return in[i].to < in[j].to
	})
	out := []portRange{in[0]}
	for _, r := range in[1:] {
		last := &out[len(out)-1]
		if r.from <= last.to+1 {
			if r.to > last.to {
				last.to = r.to
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

func formatPortCSV(ranges []portRange) string {
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r.from == r.to {
			parts = append(parts, strconv.Itoa(r.from))
			continue
		}
		parts = append(parts, strconv.Itoa(r.from)+"-"+strconv.Itoa(r.to))
	}
	return strings.Join(parts, ",")
}

func compactPortCSV(csv string) string {
	return formatPortCSV(parsePortCSV(csv))
}

func rangesCover(ranges []portRange, from, to int) bool {
	for _, r := range ranges {
		if from >= r.from && to <= r.to {
			return true
		}
	}
	return false
}

func uncoveredRanges(extra, existing []portRange) []portRange {
	var out []portRange
	for _, r := range extra {
		if !rangesCover(existing, r.from, r.to) {
			out = append(out, r)
		}
	}
	return compactRanges(out)
}

func minMaxRange(ranges []portRange) []portRange {
	if len(ranges) == 0 {
		return nil
	}
	from, to := ranges[0].from, ranges[0].to
	for _, r := range ranges[1:] {
		if r.from < from {
			from = r.from
		}
		if r.to > to {
			to = r.to
		}
	}
	return []portRange{{from, to}}
}

// Flowseal uses UDP/TCP port 12 as a dummy when Game Filter is off.
const dummyGameFilterPort = 12

func stripDummyPorts(ranges []portRange) []portRange {
	var out []portRange
	for _, r := range ranges {
		if r.from == dummyGameFilterPort && r.to == dummyGameFilterPort {
			continue
		}
		out = append(out, r)
	}
	return out
}

func rangeWidth(r portRange) int {
	return r.to - r.from + 1
}

func fitsUDPFragment(ranges []portRange, maxFragment int) bool {
	return maxFragment <= 0 || estimateUDPFilterFragment(ranges) <= maxFragment
}

// estimateUDPFilterFragment is a conservative length of winws's
// `udp.DstPort==N or (udp.DstPort>=A and udp.DstPort<=B)` fragment.
func estimateUDPFilterFragment(ranges []portRange) int {
	if len(ranges) == 0 {
		return 0
	}
	n := 0
	for i, r := range ranges {
		if i > 0 {
			n += len(" or ")
		}
		if r.from == r.to {
			n += len("udp.DstPort==") + len(strconv.Itoa(r.from))
			continue
		}
		n += len("(udp.DstPort>=") + len(strconv.Itoa(r.from)) +
			len(" and udp.DstPort<=") + len(strconv.Itoa(r.to)) + len(")")
	}
	return n
}

func mergeUDPPortCSV(existing, extra string, maxFragment int) string {
	if extra == "" {
		return compactPortCSV(existing)
	}
	have := stripDummyPorts(parsePortCSV(existing))
	add := uncoveredRanges(stripDummyPorts(parsePortCSV(extra)), have)
	if len(add) == 0 {
		if len(have) == 0 {
			return compactPortCSV(existing)
		}
		return formatPortCSV(have)
	}
	combined := compactRanges(append(append([]portRange{}, have...), add...))
	if fitsUDPFragment(combined, maxFragment) {
		return formatPortCSV(combined)
	}
	// Do not fill gaps (e.g. 3400-61457). That makes WinDivert capture
	// almost all UDP and winws exits right after start.
	picked := append([]portRange{}, have...)
	if !fitsUDPFragment(picked, maxFragment) {
		return formatPortCSV(picked)
	}
	sort.Slice(add, func(i, j int) bool {
		wi, wj := rangeWidth(add[i]), rangeWidth(add[j])
		if wi != wj {
			return wi > wj
		}
		return add[i].from < add[j].from
	})
	rest := add
	if len(add) > 0 {
		trial := compactRanges(append(append([]portRange{}, picked...), add[0]))
		if fitsUDPFragment(trial, maxFragment) {
			picked = trial
			rest = add[1:]
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		wi, wj := rangeWidth(rest[i]), rangeWidth(rest[j])
		if wi != wj {
			return wi < wj
		}
		return rest[i].from < rest[j].from
	})
	for _, r := range rest {
		trial := compactRanges(append(append([]portRange{}, picked...), r))
		if fitsUDPFragment(trial, maxFragment) {
			picked = trial
		}
	}
	return formatPortCSV(picked)
}

func mergeWfUDP(args []string, extraCSV string) []string {
	extraCSV = compactPortCSV(extraCSV)
	if extraCSV == "" {
		return args
	}
	out := append([]string(nil), args...)
	found := false
	for i, a := range out {
		if !strings.HasPrefix(a, "--wf-udp=") {
			continue
		}
		cur := strings.TrimPrefix(a, "--wf-udp=")
		out[i] = "--wf-udp=" + mergeUDPPortCSV(cur, extraCSV, maxUDPFilterFragment)
		found = true
	}
	if !found {
		out = append([]string{"--wf-udp=" + mergeUDPPortCSV("", extraCSV, maxUDPFilterFragment)}, out...)
	}
	return out
}

func joinGameUDP(games []GameStrategy) string {
	var parts []string
	for _, g := range games {
		if g.UDPPorts != "" {
			parts = append(parts, g.UDPPorts)
		}
	}
	return compactPortCSV(strings.Join(parts, ","))
}

func portCSVCovers(csv string, port int) bool {
	return rangesCover(parsePortCSV(csv), port, port)
}
