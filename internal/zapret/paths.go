package zapret

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	dataRootOnce sync.Once
	dataRootVal  string
)

func DataRoot() string {
	dataRootOnce.Do(func() {
		dataRootVal = filepath.Join(os.TempDir(), "Zapret Manager")
	})
	return dataRootVal
}

func DefaultInstallDir() string {
	return filepath.Join(DataRoot(), "zapret")
}

func CacheDir() string {
	return filepath.Join(DataRoot(), "cache")
}

func HostsConfigPath() string {
	return filepath.Join(DataRoot(), "hosts.json")
}

func LogsDir() string {
	return filepath.Join(DataRoot(), "logs")
}

func LogFile() string {
	return filepath.Join(LogsDir(), "app.log")
}

func BinDir(root string) string   { return filepath.Join(root, "bin") }
func ListsDir(root string) string { return filepath.Join(root, "lists") }
func UtilsDir(root string) string { return filepath.Join(root, "utils") }

func WinwsPath(root string) string {
	return filepath.Join(BinDir(root), "winws.exe")
}

func ServiceBat(root string) string {
	return filepath.Join(root, "service.bat")
}

func GameFilterFlag(root string) string {
	return filepath.Join(UtilsDir(root), "game_filter.enabled")
}

func IsInstalled(root string) bool {
	_, err := os.Stat(WinwsPath(root))
	return err == nil
}

func TrailingSlash(root string) string {
	root = strings.TrimRight(root, `\/`)
	return root + `\`
}

func UserListFiles() []string {
	return []string{
		filepath.Join("lists", "list-general-user.txt"),
		filepath.Join("lists", "list-exclude-user.txt"),
		filepath.Join("lists", "ipset-exclude-user.txt"),
		filepath.Join("utils", "game_filter.enabled"),
		filepath.Join("utils", "check_updates.enabled"),
	}
}
