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
	_ = os.Remove(exe + ".old")
	_ = os.Remove(exe + ".new")
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
// Returns the path to the .new file, or empty if nothing to apply.
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
	dest := exe + ".new"
	_ = os.Remove(dest)

	c := github.NewManager()
	_, urls, err := c.LatestManagerRelease(ctx, version.Version)
	if err != nil || len(urls) == 0 {
		return "", err
	}

	if err := waitNotBusy(ctx, busy); err != nil {
		return "", err
	}

	var last error
	for _, u := range urls {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := c.Download(ctx, u, dest, nil); err != nil {
			last = err
			_ = os.Remove(dest)
			continue
		}
		if err := verifyPE(dest); err != nil {
			last = err
			_ = os.Remove(dest)
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
