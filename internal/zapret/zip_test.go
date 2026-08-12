package zapret

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

func zipDir(src, zipPath, prefix string) error {
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(prefix, rel))
		if info.IsDir() {
			_, err := w.Create(name + "/")
			return err
		}
		fw, err := w.Create(name)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(fw, in)
		return err
	})
}
