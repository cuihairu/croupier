package logic

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func extractPackArchive(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.FromSlash(hdr.Name)
		if !(strings.HasPrefix(name, "descriptors/") ||
			strings.HasPrefix(name, "ui/") ||
			strings.HasPrefix(name, "web-plugin/") ||
			name == "manifest.json" ||
			strings.HasSuffix(name, ".pb")) {
			continue
		}
		if strings.HasPrefix(name, "descriptors/") {
			name = strings.TrimPrefix(name, "descriptors/")
		}
		target := filepath.Join(dest, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		out.Close()
	}
	return nil
}

func computePackETag(packDir string) string {
	h := sha256.New()
	writeFile := func(rel string) {
		data, err := os.ReadFile(filepath.Join(packDir, rel))
		if err != nil {
			return
		}
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	writeFile("manifest.json")
	_ = filepath.Walk(filepath.Join(packDir, "descriptors"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		if rel, err := filepath.Rel(packDir, path); err == nil {
			writeFile(rel)
		}
		return nil
	})
	_ = filepath.Walk(filepath.Join(packDir, "ui"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		if rel, err := filepath.Rel(packDir, path); err == nil {
			writeFile(rel)
		}
		return nil
	})
	_ = filepath.Walk(filepath.Join(packDir, "web-plugin"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || filepath.Ext(path) != ".js" {
			return nil
		}
		if rel, err := filepath.Rel(packDir, path); err == nil {
			writeFile(rel)
		}
		return nil
	})
	_ = filepath.Walk(packDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || filepath.Ext(path) != ".pb" || filepath.Dir(path) != packDir {
			return nil
		}
		if rel, err := filepath.Rel(packDir, path); err == nil {
			writeFile(rel)
		}
		return nil
	})
	return fmt.Sprintf("%x", h.Sum(nil))
}

func streamPackArchive(w io.Writer, packDir string) error {
	gz := gzip.NewWriter(w)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	return filepath.Walk(packDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(packDir, path)
		if err != nil {
			return nil
		}
		if !(strings.HasPrefix(rel, "descriptors/") ||
			strings.HasPrefix(rel, "ui/") ||
			strings.HasPrefix(rel, "web-plugin/") ||
			rel == "manifest.json" ||
			filepath.Ext(rel) == ".pb") {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, f)
		f.Close()
		return err
	})
}
