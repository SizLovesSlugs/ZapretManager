package zapret

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Strategy struct {
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
	FileName  string `json:"fileName"`
	Path      string `json:"path"`
}

func ShortName(name string) string {
	const prefix, suffix = "general (", ")"
	if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
		return strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
	}
	return name
}

const DefaultShortName = "ALT11"

// PickDefault returns the ALT11 strategy when present, otherwise the first item.
func PickDefault(strats []Strategy) string {
	if len(strats) == 0 {
		return ""
	}
	for _, s := range strats {
		if strings.EqualFold(s.ShortName, DefaultShortName) || strings.EqualFold(s.Name, DefaultShortName) {
			return s.Name
		}
	}
	return strats[0].Name
}

var continuationRe = regexp.MustCompile(`\^\s*\n`)

func ListStrategies(root string) ([]Strategy, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []Strategy
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".bat") {
			continue
		}
		if strings.HasPrefix(lower, "service") {
			continue
		}
		full := strings.TrimSuffix(name, filepath.Ext(name))
		out = append(out, Strategy{
			Name:      full,
			ShortName: ShortName(full),
			FileName:  name,
			Path:      filepath.Join(root, name),
		})
	}
	sortStrategies(out)
	return out, nil
}

func ParseWinwsArgs(batPath, root string, gf GameFilter) ([]string, error) {
	data, err := os.ReadFile(batPath)
	if err != nil {
		return nil, err
	}
	return ParseWinwsArgsContent(string(data), root, gf)
}

func ParseWinwsArgsContent(content, root string, gf GameFilter) ([]string, error) {
	flat := flattenBat(content)
	flat = expandVars(flat, root, gf)
	cmd := extractWinwsTail(flat)
	if cmd == "" {
		return nil, fmt.Errorf("winws.exe arguments not found")
	}
	args := splitArgs(cmd)
	if len(args) == 0 {
		return nil, fmt.Errorf("empty winws.exe arguments")
	}
	return args, nil
}

func flattenBat(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return continuationRe.ReplaceAllString(s, " ")
}

func expandVars(s, root string, gf GameFilter) string {
	if gf.TCP == "" {
		gf = DisabledGameFilter()
	}
	rootSlash := TrailingSlash(root)
	bin := rootSlash + `bin\`
	lists := rootSlash + `lists\`
	r := strings.NewReplacer(
		`%~dp0`, rootSlash,
		`%BIN%`, bin,
		`%LISTS%`, lists,
		`%GameFilterTCP%`, gf.TCP,
		`%GameFilterUDP%`, gf.UDP,
		`%GameFilter%`, firstNonEmpty(gf.TCP, gf.UDP, "12"),
	)
	return r.Replace(s)
}

func extractWinwsTail(s string) string {
	lower := strings.ToLower(s)
	idx := strings.LastIndex(lower, "winws.exe")
	if idx < 0 {
		return ""
	}
	rest := s[idx+len("winws.exe"):]
	rest = strings.TrimLeft(rest, `"`)
	return strings.TrimSpace(rest)
}

func splitArgs(s string) []string {
	var args []string
	var b strings.Builder
	inQuote := false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		args = append(args, b.String())
		b.Reset()
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case (c == ' ' || c == '\t' || c == '\n') && !inQuote:
			flush()
		default:
			b.WriteByte(c)
		}
	}
	flush()
	return args
}

func sortStrategies(items []Strategy) {
	sort.Slice(items, func(i, j int) bool {
		return naturalLess(items[i].Name, items[j].Name)
	})
}

func naturalLess(a, b string) bool {
	ia, ib := 0, 0
	for ia < len(a) && ib < len(b) {
		ca, cb := rune(a[ia]), rune(b[ib])
		if unicode.IsDigit(ca) && unicode.IsDigit(cb) {
			na, naLen := readNum(a[ia:])
			nb, nbLen := readNum(b[ib:])
			if na != nb {
				return na < nb
			}
			ia += naLen
			ib += nbLen
			continue
		}
		la, lb := unicode.ToLower(ca), unicode.ToLower(cb)
		if la != lb {
			return la < lb
		}
		ia++
		ib++
	}
	return len(a) < len(b)
}

func readNum(s string) (int, int) {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	v, _ := strconv.Atoi(s[:n])
	return v, n
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
