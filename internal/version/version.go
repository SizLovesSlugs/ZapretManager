package version

import (
	"fmt"
	"strings"
)

// Version is the Zapret Manager app version shown in the window title.
const Version = "1.0 Beta"

// FileVersion is the 4-part Windows VERSIONINFO numeric form.
var FileVersion = [4]uint16{1, 0, 0, 0}

func Title() string {
	return "Zapret Manager " + Version
}

// DisplayVersion turns a git tag into the human version used in the exe name.
// "1.0-Beta" / "v1.0-Beta" -> "1.0 Beta".
func DisplayVersion(tag string) string {
	v := strings.TrimSpace(tag)
	v = strings.TrimPrefix(v, "v")
	v = strings.ReplaceAll(v, "-", " ")
	return v
}

// ExeName is the output binary filename, e.g. "Zapret Manager 1.0 Beta.exe".
func ExeName() string {
	return ExeNameFor(Version)
}

// ExeNameFor is the release asset name for a tag or version string.
func ExeNameFor(tag string) string {
	return "Zapret Manager " + DisplayVersion(tag) + ".exe"
}

// FileVersionString is x.y.z for Windows string table (goversioninfo requires 3+ parts).
func FileVersionString() string {
	return fmt.Sprintf("%d.%d.%d", FileVersion[0], FileVersion[1], FileVersion[2])
}
