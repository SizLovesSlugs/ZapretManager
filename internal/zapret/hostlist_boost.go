package zapret

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	boostHostlistBegin = "# BEGIN Zapret Manager boost-hostlist"
	boostHostlistEnd   = "# END Zapret Manager boost-hostlist"
)

func GeneralUserListPath(root string) string {
	return filepath.Join(ListsDir(root), "list-general-user.txt")
}

// SyncBoostHostlist writes enabled boost domains into list-general-user.txt.
// Flowseal Windows strategies use --hostlist (allowlist), unlike ZMS v7 which
// uses --hostlist-exclude (desync almost all TCP/443). Without these domains
// in the allowlist, hosts IP rewrite alone is not enough for SNI DPI.
func SyncBoostHostlist(root string, domains []string) (bool, error) {
	if err := EnsureUserLists(root); err != nil {
		return false, err
	}
	path := GeneralUserListPath(root)
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		raw = []byte("# Never leave this file empty\ndomain.example.abc\n")
	}
	next := applyBoostHostlistBlock(string(raw), domains)
	if next == string(raw) {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func ClearBoostHostlist(root string) error {
	_, err := SyncBoostHostlist(root, nil)
	return err
}

func applyBoostHostlistBlock(content string, domains []string) string {
	nl := "\n"
	if strings.Contains(content, "\r\n") {
		nl = "\r\n"
	}
	cleaned := stripMarkedBlock(content, boostHostlistBegin, boostHostlistEnd)
	cleaned = strings.TrimRight(cleaned, " \t\r\n")
	block := buildBoostHostlistBlock(domains, nl)
	if block == "" {
		if cleaned == "" {
			return "# Never leave this file empty" + nl + "domain.example.abc" + nl
		}
		return cleaned + nl
	}
	if cleaned == "" {
		return block
	}
	return cleaned + nl + nl + block
}

func buildBoostHostlistBlock(domains []string, nl string) string {
	seen := map[string]bool{}
	var lines []string
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || strings.HasPrefix(d, "#") || seen[d] {
			continue
		}
		seen[d] = true
		lines = append(lines, d)
	}
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(boostHostlistBegin)
	b.WriteString(nl)
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString(nl)
	}
	b.WriteString(boostHostlistEnd)
	b.WriteString(nl)
	return b.String()
}

func stripMarkedBlock(content, begin, end string) string {
	for {
		start := strings.Index(content, begin)
		if start < 0 {
			return content
		}
		endIdx := strings.Index(content[start:], end)
		if endIdx < 0 {
			return strings.TrimRight(content[:start], " \t\r\n")
		}
		endPos := start + endIdx + len(end)
		for endPos < len(content) && (content[endPos] == '\n' || content[endPos] == '\r') {
			endPos++
		}
		before := strings.TrimRight(content[:start], " \t\r\n")
		after := strings.TrimLeft(content[endPos:], " \t")
		if after != "" && (strings.HasPrefix(after, "\r\n") || strings.HasPrefix(after, "\n")) {
			after = strings.TrimLeft(after, "\r\n")
		}
		if before == "" {
			content = after
			continue
		}
		if after == "" {
			content = before
			continue
		}
		nl := "\n"
		if strings.Contains(content, "\r\n") {
			nl = "\r\n"
		}
		content = before + nl + after
	}
}
