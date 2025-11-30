package pack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
)

type packSummary struct {
	Name            string
	Path            string
	Manifest        map[string]interface{}
	DescriptorCount int
	UISchemaCount   int
	UpdatedAt       time.Time
}

func resolvePacksDir(cfg config.Config) string {
	dir := strings.TrimSpace(cfg.Packs.Dir)
	if dir == "" {
		return "packs"
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func loadPackSummaries(baseDir string) ([]packSummary, error) {
	dirEntries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}
	summaries := make([]packSummary, 0, len(dirEntries))
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		packPath := filepath.Join(baseDir, name)
		manifest, modTime := readManifest(filepath.Join(packPath, "manifest.json"))
		descCount := countJSONFiles(filepath.Join(packPath, "descriptors"))
		uiCount := countJSONFiles(filepath.Join(packPath, "ui"))
		if modTime.IsZero() {
			if info, err := entry.Info(); err == nil {
				modTime = info.ModTime()
			} else {
				modTime = time.Now()
			}
		}
		summaries = append(summaries, packSummary{
			Name:            name,
			Path:            packPath,
			Manifest:        manifest,
			DescriptorCount: descCount,
			UISchemaCount:   uiCount,
			UpdatedAt:       modTime,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})
	return summaries, nil
}

func readManifest(path string) (map[string]interface{}, time.Time) {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]interface{}{}, time.Time{}
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return map[string]interface{}{}, time.Time{}
	}
	info, _ := os.Stat(path)
	var modTime time.Time
	if info != nil {
		modTime = info.ModTime()
	}
	return manifest, modTime
}

func countJSONFiles(dir string) int {
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, f := range files {
		if f.IsDir() {
			count += countJSONFiles(filepath.Join(dir, f.Name()))
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Name()), ".json") {
			count++
		}
	}
	return count
}

func aggregateManifest(summaries []packSummary) (map[string]interface{}, []map[string]interface{}) {
	packs := make([]map[string]interface{}, 0, len(summaries))
	functions := make([]interface{}, 0)
	webPlugins := make([]interface{}, 0)

	for _, summary := range summaries {
		packEntry := map[string]interface{}{
			"id":          summary.Name,
			"descriptors": summary.DescriptorCount,
			"ui_schema":   summary.UISchemaCount,
			"updated_at":  utils.FormatTimestamp(summary.UpdatedAt),
			"manifest":    summary.Manifest,
		}
		packs = append(packs, packEntry)

		if fnList, ok := summary.Manifest["functions"].([]interface{}); ok {
			functions = append(functions, fnList...)
		}
		if plugins, ok := summary.Manifest["web_plugins"].([]interface{}); ok {
			webPlugins = append(webPlugins, plugins...)
		}
	}

	manifest := map[string]interface{}{
		"packs":       packs,
		"functions":   functions,
		"web_plugins": webPlugins,
	}

	return manifest, packs
}

func buildPacksArchive(baseDir string) (string, []byte, error) {
	archiveRoot := filepath.Join(baseDir, "dist")
	if info, err := os.Stat(archiveRoot); err != nil || !info.IsDir() {
		archiveRoot = baseDir
	}

	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	err := filepath.WalkDir(archiveRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(archiveRoot, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := io.Copy(tw, file); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		tw.Close()
		gw.Close()
		return "", nil, err
	}
	if err := tw.Close(); err != nil {
		gw.Close()
		return "", nil, err
	}
	if err := gw.Close(); err != nil {
		return "", nil, err
	}

	filename := fmt.Sprintf("packs-export-%d.tar.gz", time.Now().Unix())
	return filename, buf.Bytes(), nil
}

func extractArchive(data []byte, destDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			continue
		}
		targetPath := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(destDir)) {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		out, err := os.Create(targetPath)
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
