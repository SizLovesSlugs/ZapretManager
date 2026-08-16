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

// Prepare silently checks GitHub and downloads a newer manager exe next to the running file.
// The downloaded file is named for the new version, e.g. ZapretManager-1.1.2.exe.
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
	tag, urls, err := c.LatestManagerRelease(ctx, version.Version)
	if err != nil {
		return "", err
	}
	if tag == "" || len(urls) == 0 {
		return "", nil
	}

	if err := waitNotBusy(ctx, busy); err != nil {
		return "", err
	}

	dest := filepath.Join(filepath.Dir(exe), version.ExeNameFor(tag))
	if strings.EqualFold(filepath.Clean(dest), filepath.Clean(exe)) {
		dest = exe + ".new"
	}
	tmp := dest + ".part"
	_ = os.Remove(tmp)
	_ = os.Remove(dest)

	var last error
	for _, u := range urls {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := c.Download(ctx, u, tmp, nil); err != nil {
			last = err
			_ = os.Remove(tmp)
			continue
		}
		if err := verifyPE(tmp); err != nil {
			last = err
			_ = os.Remove(tmp)
			continue
		}
		_ = os.Remove(dest)
		if err := os.Rename(tmp, dest); err != nil {
			last = err
			_ = os.Remove(tmp)
			continue
		}
		return dest, nil
	}
	return "", last
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
