package version

import "fmt"

// Version is the Zapret Manager app version shown in the window title.
const Version = "0.1"

// FileVersion is the 4-part Windows VERSIONINFO numeric form.
var FileVersion = [4]uint16{0, 1, 0, 0}

func Title() string {
	return "Zapret Manager " + Version
}

// ExeName is the output binary filename, e.g. "Zapret Manager 0.1.exe".
func ExeName() string {
	return Title() + ".exe"
}

// FileVersionString is x.y.z for Windows string table (goversioninfo requires 3+ parts).
func FileVersionString() string {
	return fmt.Sprintf("%d.%d.%d", FileVersion[0], FileVersion[1], FileVersion[2])
}
