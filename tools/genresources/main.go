// Generates assets/logo.ico and cmd/zapret-manager/resource.syso from logo.svg.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	"github.com/josephspurrier/goversioninfo"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"zapret-manager/internal/version"
)

func main() {
	root, err := findRoot()
	if err != nil {
		fatal(err)
	}
	svgPath := filepath.Join(root, "logo.svg")
	assetsDir := filepath.Join(root, "assets")
	icoPath := filepath.Join(assetsDir, "logo.ico")
	sysoPath := filepath.Join(root, "cmd", "zapret-manager", "resource.syso")

	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		fatal(err)
	}
	if _, err := os.Stat(svgPath); err == nil {
		sizes := []int{16, 24, 32, 48, 64, 128, 256}
		imgs, err := renderSVG(svgPath, sizes)
		if err != nil {
			fatal(err)
		}
		if err := writeICO(icoPath, imgs); err != nil {
			fatal(err)
		}
		fmt.Println("wrote", icoPath)
	} else if _, err := os.Stat(icoPath); err != nil {
		fatal(fmt.Errorf("logo.svg and assets/logo.ico not found (root %s)", root))
	} else {
		fmt.Println("using existing", icoPath)
	}

	if err := writeSyso(icoPath, sysoPath); err != nil {
		fatal(err)
	}
	fmt.Println("wrote", sysoPath)
}

func findRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return wd, nil
}

func renderSVG(path string, sizes []int) ([]image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	icon, err := oksvg.ReadIconStream(f, oksvg.StrictErrorMode)
	if err != nil {
		return nil, err
	}
	out := make([]image.Image, 0, len(sizes))
	for _, size := range sizes {
		icon.SetTarget(0, 0, float64(size), float64(size))
		rgba := image.NewRGBA(image.Rect(0, 0, size, size))
		draw.Draw(rgba, rgba.Bounds(), image.Transparent, image.Point{}, draw.Src)
		scanner := rasterx.NewScannerGV(size, size, rgba, rgba.Bounds())
		raster := rasterx.NewDasher(size, size, scanner)
		icon.Draw(raster, 1)
		out = append(out, rgba)
	}
	return out, nil
}

func writeICO(path string, images []image.Image) error {
	type entry struct {
		width  byte
		height byte
		data   []byte
	}
	entries := make([]entry, 0, len(images))
	for _, img := range images {
		b := img.Bounds()
		w, h := b.Dx(), b.Dy()
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return err
		}
		wb, hb := byte(w), byte(h)
		if w >= 256 {
			wb = 0
		}
		if h >= 256 {
			hb = 0
		}
		entries = append(entries, entry{width: wb, height: hb, data: buf.Bytes()})
	}

	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(0)) // reserved
	_ = binary.Write(&out, binary.LittleEndian, uint16(1)) // type icon
	_ = binary.Write(&out, binary.LittleEndian, uint16(len(entries)))

	offset := 6 + 16*len(entries)
	for _, e := range entries {
		out.WriteByte(e.width)
		out.WriteByte(e.height)
		out.WriteByte(0) // colors
		out.WriteByte(0) // reserved
		_ = binary.Write(&out, binary.LittleEndian, uint16(1)) // planes
		_ = binary.Write(&out, binary.LittleEndian, uint16(32))
		_ = binary.Write(&out, binary.LittleEndian, uint32(len(e.data)))
		_ = binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(e.data)
	}
	for _, e := range entries {
		out.Write(e.data)
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func writeSyso(icoPath, sysoPath string) error {
	v := version.FileVersion
	vi := &goversioninfo.VersionInfo{
		FixedFileInfo: goversioninfo.FixedFileInfo{
			FileVersion:    goversioninfo.FileVersion{Major: int(v[0]), Minor: int(v[1]), Patch: int(v[2]), Build: int(v[3])},
			ProductVersion: goversioninfo.FileVersion{Major: int(v[0]), Minor: int(v[1]), Patch: int(v[2]), Build: int(v[3])},
			FileFlagsMask:  "3f",
			FileFlags:      "00",
			FileOS:         "040004",
			FileType:       "01",
			FileSubType:    "00",
		},
		StringFileInfo: goversioninfo.StringFileInfo{
			Comments:         "Zapret Manager by Siz",
			CompanyName:      "Siz",
			FileDescription:  "Zapret Manager",
			FileVersion:      version.FileVersionString(),
			InternalName:     "zapret-manager",
			LegalCopyright:   "Made by Siz Loves Slugs",
			OriginalFilename: version.ExeName(),
			ProductName:      "Zapret Manager",
			ProductVersion:   version.FileVersionString(),
		},
		VarFileInfo: goversioninfo.VarFileInfo{
			Translation: goversioninfo.Translation{LangID: 0x0409, CharsetID: 0x04B0},
		},
		IconPath: icoPath,
	}
	vi.Build()
	vi.Walk()
	return vi.WriteSyso(sysoPath, "amd64")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
