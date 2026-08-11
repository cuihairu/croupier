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

	"github.com/cuihairu/croupier/internal/function/descriptor"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func main() {
	var (
		path     = flag.String("path", ".", "Path to validate (file or directory)")
		packPath = flag.String("pack", "", "Path to pack file (.tgz)")
		verbose  = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	if *packPath != "" {
		if err := validatePack(*packPath, *verbose); err != nil {
			log.Fatalf("Pack validation failed: %v", err)
		}
		fmt.Println("✅ Pack validation passed")
		return
	}

	if err := validatePath(*path, *verbose); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}
	fmt.Println("✅ Validation passed")
}

func validatePath(path string, verbose bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access path: %w", err)
	}

	if info.IsDir() {
		return validateDirectory(path, verbose)
	}
	return validateFile(path, verbose)
}

func validateDirectory(dir string, verbose bool) error {
	if verbose {
		fmt.Printf("🔍 Validating directory: %s\n", dir)
	}

	var errors []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip non-JSON files
		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		if verbose {
			fmt.Printf("  📄 Checking: %s\n", path)
		}

		if err := validateFile(path, false); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
		}

		return nil
	})

	if err != nil {
		return err
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

func validateFile(filePath string, verbose bool) error {
	if verbose {
		fmt.Printf("🔍 Validating file: %s\n", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	// Determine file type by path and content
	fileName := filepath.Base(filePath)
	dir := filepath.Dir(filePath)

	switch {
	case strings.Contains(dir, "descriptors") && !strings.Contains(fileName, "ui."):
		return validateDescriptor(data, verbose)
	case strings.Contains(fileName, "ui.") || strings.Contains(fileName, ".ui."):
		return validateUISchema(data, verbose)
	case fileName == "manifest.json":
		return validateManifest(data, verbose)
	default:
		// Try to guess by content structure
		var obj map[string]interface{}
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}

		if _, hasParams := obj["params"]; hasParams {
			return validateDescriptor(data, verbose)
		}
		if _, hasWidget := obj["widget"]; hasWidget {
			return validateUISchema(data, verbose)
		}
		if _, hasName := obj["name"]; hasName {
			return validateManifest(data, verbose)
		}

		if verbose {
			fmt.Printf("  ⚠️  Unknown file type, skipping validation\n")
		}
	}

	return nil
}

func validateDescriptor(data []byte, verbose bool) error {
	// Parse as descriptor
	var desc descriptor.Descriptor
	if err := json.Unmarshal(data, &desc); err != nil {
		return fmt.Errorf("invalid descriptor JSON: %w", err)
	}

	// Basic descriptor validation
	if desc.ID == "" {
		return fmt.Errorf("descriptor missing required field: id")
	}
	if desc.Version == "" {
		return fmt.Errorf("descriptor missing required field: version")
	}

	// Validate JSON Schema if params exist
	if desc.Params != nil {
		if err := validateJSONSchema(desc.Params); err != nil {
			return fmt.Errorf("invalid params schema: %w", err)
		}
	}

	// Check for outputs (results) validation
	if desc.Outputs != nil {
		if err := validateJSONSchema(desc.Outputs); err != nil {
			return fmt.Errorf("invalid outputs schema: %w", err)
		}
	}

	if verbose {
		fmt.Printf("    ✅ Valid descriptor: %s v%s\n", desc.ID, desc.Version)
	}

	return nil
}

func validateUISchema(data []byte, verbose bool) error {
	var uiSchema map[string]interface{}
	if err := json.Unmarshal(data, &uiSchema); err != nil {
		return fmt.Errorf("invalid UI schema JSON: %w", err)
	}

	// Basic UI schema validation
	if verbose {
		fmt.Printf("    ✅ Valid UI schema\n")
	}

	return nil
}

func validateManifest(data []byte, verbose bool) error {
	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("invalid manifest JSON: %w", err)
	}

	// Basic manifest validation
	requiredFields := []string{"name", "version", "descriptors"}
	for _, field := range requiredFields {
		if _, exists := manifest[field]; !exists {
			return fmt.Errorf("manifest missing required field: %s", field)
		}
	}

	if verbose {
		fmt.Printf("    ✅ Valid manifest: %s v%s\n", manifest["name"], manifest["version"])
	}

	return nil
}

func validateJSONSchema(schema interface{}) error {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	if err := compiler.AddResource("schema.json", schema); err != nil {
		return err
	}
	_, err := compiler.Compile("schema.json")
	return err
}

func validatePack(packPath string, verbose bool) error {
	if verbose {
		fmt.Printf("🔍 Validating pack: %s\n", packPath)
	}

	tempDir, err := os.MkdirTemp("", "croupier-pack-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := extractTarGz(packPath, tempDir); err != nil {
		return fmt.Errorf("failed to extract pack: %w", err)
	}

	manifestPath := filepath.Join(tempDir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("pack missing manifest.json: %w", err)
	}

	if err := validateDirectory(tempDir, verbose); err != nil {
		return fmt.Errorf("pack content validation failed: %w", err)
	}

	return nil
}

func extractTarGz(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open pack: %w", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("invalid gzip stream: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("cannot resolve destination: %w", err)
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar iteration error: %w", err)
		}

		// Clean the name and reject path traversal attempts before joining
		cleanName := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanName, "..") || strings.Contains(cleanName, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("path traversal detected in archive: %s", hdr.Name)
		}
		target := filepath.Join(destAbs, cleanName)
		if !strings.HasPrefix(target, destAbs+string(os.PathSeparator)) && target != destAbs {
			return fmt.Errorf("invalid path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("failed to create dir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("failed to create parent dir for %s: %w", target, err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", target, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("failed to extract %s: %w", target, err)
			}
			out.Close()
		case tar.TypeSymlink, tar.TypeLink:
			// skip links for safety
			continue
		default:
			// ignore other types
			continue
		}
	}

	return nil
}
