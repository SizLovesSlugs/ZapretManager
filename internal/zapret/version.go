package zapret

import (
	"os"
	"regexp"
	"strings"
)

var localVersionRe = regexp.MustCompile(`(?i)set\s+"LOCAL_VERSION=([^"]+)"`)

func LocalVersion(root string) string {
	data, err := os.ReadFile(ServiceBat(root))
	if err != nil {
		return ""
	}
	m := localVersionRe.FindSubmatch(data)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}
