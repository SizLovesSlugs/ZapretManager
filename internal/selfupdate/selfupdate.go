package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zapret-manager/internal/github"
	"zapret-manager/internal/version"
)

const minExeBytes = 1 << 20

func Cleanup() {
	exe, err := currentExe()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	_ = os.Remove(exe + ".old")
	_ = os.Remove(exe + ".new")
	_ = os.Remove(exe + ".part")
	for _, pattern := range []string{"ZapretManager-*.exe.old", "ZapretManager-*.exe.new", "ZapretManager-*.exe.part"} {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
}

func CanCheck() bool {
	exe, err := currentExe()
	if err != nil {
		return false
	}
	lower := strings.ToLower(exe)
	if strings.Contains(lower, "go-build") || strings.Contains(lower, "go-tmp") {
		return false
	}
	return true
}

func currentExe() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Abs(exe)
}

func waitNotBusy(ctx context.Context, busy func() bool) error {
	deadline := time.Now().Add(3 * time.Minute)
	for busy() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	return ctx.Err()
}

// Prepare silently checks GitHub latest exe sha256 and downloads it if different.
// The downloaded file keeps the GitHub asset name, e.g. ZapretManager-1.1.6.exe.
func Prepare(ctx context.Context, busy func() bool) (string, error) {
	if !CanCheck() {
		return "", nil
	}
	if err := waitNotBusy(ctx, busy); err != nil {
		return "", err
	}

	exe, err := currentExe()
	if err != nil {
		return "", err
	}

	c := github.NewManager()
	asset, err := c.LatestManagerAsset(ctx)
	if err != nil {
		return "", err
	}
	if asset.URL == "" || asset.Digest == "" {
		return "", nil
	}
	local, err := github.FileSHA256(exe)
	if err != nil {
		return "", err
	}
	if github.SameDigest(local, asset.Digest) {
		return "", nil
	}

	if err := waitNotBusy(ctx, busy); err != nil {
		return "", err
	}

	name := strings.TrimSpace(asset.Name)
	if name == "" {
		name = version.ExeNameFor(asset.Tag)
	}
	dest := filepath.Join(filepath.Dir(exe), name)
	if strings.EqualFold(filepath.Clean(dest), filepath.Clean(exe)) {
		dest = exe + ".new"
	}
	tmp := dest + ".part"
	_ = os.Remove(tmp)
	_ = os.Remove(dest)

	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := c.Download(ctx, asset.URL, tmp, nil); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := verifyPE(tmp); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := github.VerifySHA256(tmp, asset.Digest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return dest, nil
}

func verifyPE(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() < minExeBytes {
		return os.ErrInvalid
	}
	var hdr [2]byte
	if _, err := f.Read(hdr[:]); err != nil {
		return err
	}
	if hdr[0] != 'M' || hdr[1] != 'Z' {
		return os.ErrInvalid
	}
	return nil
}
