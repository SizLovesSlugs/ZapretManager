package version

import (
	"fmt"
	"strings"
)

// Version is the Zapret Manager app version shown in the window title.
const Version = "1.0"

// FileVersion is the 4-part Windows VERSIONINFO numeric form.
var FileVersion = [4]uint16{1, 0, 0, 0}

func Title() string {
	return "Zapret Manager " + Version
}

// VersionTag turns a git tag into the version fragment used in the exe name.
// "v1.0" / "1.0 Beta" / "1.0-Beta" -> "1.0" / "1.0-Beta".
func VersionTag(tag string) string {
	v := strings.TrimSpace(tag)
	v = strings.TrimPrefix(v, "v")
	v = strings.ReplaceAll(v, " ", "-")
	return v
}

// ExeName is the output binary filename, e.g. "ZapretManager-1.0.exe".
func ExeName() string {
	return ExeNameFor(Version)
}

// ExeNameFor is the release asset name for a tag or version string.
func ExeNameFor(tag string) string {
	return "ZapretManager-" + VersionTag(tag) + ".exe"
}

// FileVersionString is x.y.z for Windows string table (goversioninfo requires 3+ parts).
func FileVersionString() string {
	return fmt.Sprintf("%d.%d.%d", FileVersion[0], FileVersion[1], FileVersion[2])
}
