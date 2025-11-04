package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Manifest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Author      string            `json:"author,omitempty"`
	Descriptors []string          `json:"descriptors"`
	UI          map[string]string `json:"ui,omitempty"`
	Assets      []string          `json:"assets,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func main() {
	var (
		input    = flag.String("input", ".", "Input directory containing descriptors")
		output   = flag.String("output", "", "Output pack file (.tgz)")
		name     = flag.String("name", "", "Pack name (required)")
		version  = flag.String("version", "1.0.0", "Pack version")
		desc     = flag.String("desc", "", "Pack description")
		author   = flag.String("author", "", "Pack author")
		validate = flag.Bool("validate", true, "Validate before building")
		verbose  = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	if *name == "" {
		log.Fatal("Pack name is required (use -name)")
	}

	if *output == "" {
		*output = fmt.Sprintf("%s-%s.tgz", *name, *version)
	}

	builder := &PackBuilder{
		InputDir:  *input,
		OutputFile: *output,
		Name:      *name,
		Version:   *version,
		Description: *desc,
		Author:    *author,
		Validate:  *validate,
		Verbose:   *verbose,
	}

	if err := builder.Build(); err != nil {
		log.Fatalf("Build failed: %v", err)
	}

	fmt.Printf("✅ Pack built successfully: %s\n", *output)
}

type PackBuilder struct {
	InputDir    string
	OutputFile  string
	Name        string
	Version     string
	Description string
	Author      string
	Validate    bool
	Verbose     bool
}

func (pb *PackBuilder) Build() error {
	if pb.Verbose {
		fmt.Printf("🔧 Building pack: %s v%s\n", pb.Name, pb.Version)
		fmt.Printf("   Input: %s\n", pb.InputDir)
		fmt.Printf("   Output: %s\n", pb.OutputFile)
	}

	// Discover files
	manifest, err := pb.discoverFiles()
	if err != nil {
		return fmt.Errorf("failed to discover files: %w", err)
	}

	// Validate if requested
	if pb.Validate {
		if err := pb.validateFiles(manifest); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	// Create pack
	if err := pb.createPack(manifest); err != nil {
		return fmt.Errorf("failed to create pack: %w", err)
	}

	return nil
}

func (pb *PackBuilder) discoverFiles() (*Manifest, error) {
	manifest := &Manifest{
		Name:        pb.Name,
		Version:     pb.Version,
		Description: pb.Description,
		Author:      pb.Author,
		Descriptors: []string{},
		UI:          make(map[string]string),
		Assets:      []string{},
		Metadata:    make(map[string]string),
	}

	err := filepath.Walk(pb.InputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(pb.InputDir, path)
		if err != nil {
			return err
		}

		fileName := filepath.Base(path)

		switch {
		case strings.HasSuffix(fileName, ".json") && !strings.Contains(fileName, "ui."):
			// Descriptor file
			if pb.isDescriptor(path) {
				manifest.Descriptors = append(manifest.Descriptors, relPath)
				if pb.Verbose {
					fmt.Printf("   📄 Found descriptor: %s\n", relPath)
				}
			}
		case strings.Contains(fileName, "ui.") && strings.HasSuffix(fileName, ".json"):
			// UI schema file
			descriptorName := strings.Replace(fileName, ".ui.json", "", 1)
			manifest.UI[descriptorName] = relPath
			if pb.Verbose {
				fmt.Printf("   🎨 Found UI schema: %s -> %s\n", descriptorName, relPath)
			}
		case fileName == "manifest.json":
			// Skip existing manifest
		default:
			// Asset file
			manifest.Assets = append(manifest.Assets, relPath)
			if pb.Verbose {
				fmt.Printf("   📦 Found asset: %s\n", relPath)
			}
		}

		return nil
	})

	return manifest, err
}

func (pb *PackBuilder) isDescriptor(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return false
	}

	// Check if it has descriptor-like structure
	_, hasID := obj["id"]
	_, hasVersion := obj["version"]
	_, hasParams := obj["params"]

	return hasID && hasVersion && hasParams
}

func (pb *PackBuilder) validateFiles(manifest *Manifest) error {
	if pb.Verbose {
		fmt.Printf("🔍 Validating files...\n")
	}

	// Validate descriptors
	for _, desc := range manifest.Descriptors {
		filePath := filepath.Join(pb.InputDir, desc)
		if err := pb.validateDescriptor(filePath); err != nil {
			return fmt.Errorf("descriptor %s: %w", desc, err)
		}
	}

	// Validate UI schemas
	for desc, uiPath := range manifest.UI {
		filePath := filepath.Join(pb.InputDir, uiPath)
		if err := pb.validateUISchema(filePath); err != nil {
			return fmt.Errorf("UI schema %s: %w", desc, err)
		}
	}

	return nil
}

func (pb *PackBuilder) validateDescriptor(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var desc map[string]interface{}
	if err := json.Unmarshal(data, &desc); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Basic validation
	requiredFields := []string{"id", "version"}
	for _, field := range requiredFields {
		if _, exists := desc[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	return nil
}

func (pb *PackBuilder) validateUISchema(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var uiSchema map[string]interface{}
	if err := json.Unmarshal(data, &uiSchema); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	return nil
}

func (pb *PackBuilder) createPack(manifest *Manifest) error {
	if pb.Verbose {
		fmt.Printf("📦 Creating pack archive...\n")
	}

	// Create output file
	outFile, err := os.Create(pb.OutputFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add manifest.json first
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	if err := pb.addFile(tarWriter, "manifest.json", manifestData); err != nil {
		return err
	}

	// Add descriptors
	for _, desc := range manifest.Descriptors {
		if err := pb.addFileFromPath(tarWriter, desc, filepath.Join(pb.InputDir, desc)); err != nil {
			return err
		}
	}

	// Add UI schemas
	for _, uiPath := range manifest.UI {
		if err := pb.addFileFromPath(tarWriter, uiPath, filepath.Join(pb.InputDir, uiPath)); err != nil {
			return err
		}
	}

	// Add assets
	for _, asset := range manifest.Assets {
		if err := pb.addFileFromPath(tarWriter, asset, filepath.Join(pb.InputDir, asset)); err != nil {
			return err
		}
	}

	return nil
}

func (pb *PackBuilder) addFile(tarWriter *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name:    name,
		Mode:    0644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	_, err := tarWriter.Write(data)
	return err
}

func (pb *PackBuilder) addFileFromPath(tarWriter *tar.Writer, name, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    name,
		Mode:    0644,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, file)
	if pb.Verbose {
		fmt.Printf("   ✅ Added: %s\n", name)
	}
	return err
}