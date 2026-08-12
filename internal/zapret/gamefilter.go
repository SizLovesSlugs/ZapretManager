package zapret

import (
	"os"
	"strings"
)

type GameFilter struct {
	Mode string `json:"mode"` // off, all, tcp, udp
	TCP  string `json:"tcp"`
	UDP  string `json:"udp"`
}

func (g GameFilter) Label() string {
	switch g.Mode {
	case "all":
		return "TCP+UDP"
	case "tcp":
		return "TCP"
	case "udp":
		return "UDP"
	default:
		return "выкл"
	}
}

func DisabledGameFilter() GameFilter {
	return GameFilter{Mode: "off", TCP: "12", UDP: "12"}
}

func LoadGameFilter(root string) GameFilter {
	data, err := os.ReadFile(GameFilterFlag(root))
	if err != nil {
		return DisabledGameFilter()
	}
	mode := strings.ToLower(strings.TrimSpace(string(data)))
	switch mode {
	case "all":
		return GameFilter{Mode: "all", TCP: "1024-65535", UDP: "1024-65535"}
	case "tcp":
		return GameFilter{Mode: "tcp", TCP: "1024-65535", UDP: "12"}
	case "udp":
		return GameFilter{Mode: "udp", TCP: "12", UDP: "1024-65535"}
	default:
		return DisabledGameFilter()
	}
}

func SaveGameFilter(root string, mode string) error {
	if err := os.MkdirAll(UtilsDir(root), 0o755); err != nil {
		return err
	}
	path := GameFilterFlag(root)
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" || mode == "off" || mode == "disabled" {
		_ = os.Remove(path)
		InvalidateInstallCache()
		return nil
	}
	switch mode {
	case "all", "tcp", "udp":
	default:
		return os.ErrInvalid
	}
	err := os.WriteFile(path, []byte(mode+"\n"), 0o644)
	InvalidateInstallCache()
	return err
}
