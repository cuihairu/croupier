package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/function/descriptor"
	"github.com/xeipuuv/gojsonschema"
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
	// Convert to JSON and validate as JSON Schema
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return err
	}

	loader := gojsonschema.NewBytesLoader(schemaBytes)
	_, err = gojsonschema.NewSchema(loader)
	return err
}

func validatePack(packPath string, verbose bool) error {
	if verbose {
		fmt.Printf("🔍 Validating pack: %s\n", packPath)
	}

	// TODO: Implement pack validation
	// 1. Extract pack to temp directory
	// 2. Validate manifest.json
	// 3. Validate all descriptors
	// 4. Validate UI schemas
	// 5. Check file consistency

	return fmt.Errorf("pack validation not yet implemented")
}