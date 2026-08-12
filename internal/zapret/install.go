package zapret

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func InstallZip(zipPath, dest string) error {
	tmp := dest + ".new"
	_ = os.RemoveAll(tmp)
	if err := extractZip(zipPath, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	backup, err := backupUserFiles(dest)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("replace install dir: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("move install dir: %w", err)
	}
	if err := restoreUserFiles(dest, backup); err != nil {
		return err
	}
	InvalidateInstallCache()
	return EnsureUserLists(dest)
}

func EnsureUserLists(root string) error {
	if err := os.MkdirAll(ListsDir(root), 0o755); err != nil {
		return err
	}
	files := map[string]string{
		filepath.Join(ListsDir(root), "list-general-user.txt"):  "# Never leave this file empty\ndomain.example.abc\n",
		filepath.Join(ListsDir(root), "list-exclude-user.txt"):  "domain.example.abc\n",
		filepath.Join(ListsDir(root), "ipset-exclude-user.txt"): "203.0.113.113/32\n",
	}
	for path, content := range files {
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func extractZip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	prefix := commonPrefix(r.File)
	for _, f := range r.File {
		name := f.Name
		if prefix != "" {
			name = strings.TrimPrefix(name, prefix)
		}
		name = strings.TrimPrefix(name, "/")
		name = strings.TrimPrefix(name, `\`)
		if name == "" {
			continue
		}
		if err := extractFile(f, dest, name); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, dest, name string) error {
	clean := filepath.Clean(name)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return fmt.Errorf("illegal path in archive: %s", name)
	}
	target := filepath.Join(dest, clean)
	if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) && target != filepath.Clean(dest) {
		return fmt.Errorf("illegal path in archive: %s", name)
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

func commonPrefix(files []*zip.File) string {
	var prefix string
	set := false
	for _, f := range files {
		name := strings.ReplaceAll(f.Name, `\`, "/")
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 {
			return ""
		}
		if !set {
			prefix = parts[0]
			set = true
			continue
		}
		if parts[0] != prefix {
			return ""
		}
	}
	if !set || prefix == "" {
		return ""
	}
	return prefix + "/"
}

func backupUserFiles(root string) (map[string][]byte, error) {
	out := map[string][]byte{}
	if _, err := os.Stat(root); err != nil {
		return out, nil
	}
	for _, rel := range UserListFiles() {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		out[rel] = data
	}
	return out, nil
}

func restoreUserFiles(root string, files map[string][]byte) error {
	for rel, data := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
