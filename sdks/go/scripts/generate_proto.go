package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// generateGRPCCode generates Go gRPC code from proto files
func generateGRPCCode(protoDir, genDir string) error {
	fmt.Printf("Generating gRPC code from %s to %s...\n", protoDir, genDir)

	// Create generated directory
	if err := os.MkdirAll(genDir, 0755); err != nil {
		return fmt.Errorf("failed to create generated directory: %w", err)
	}

	// Find all proto files
	var protoFiles []string
	err := filepath.Walk(protoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".proto") {
			protoFiles = append(protoFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to find proto files: %w", err)
	}

	if len(protoFiles) == 0 {
		return fmt.Errorf("no proto files found in %s", protoDir)
	}

	// Generate Go code for each proto file
	for _, protoFile := range protoFiles {
		fmt.Printf("Generating code for: %s\n", protoFile)

		// Generate protobuf code using fixed version plugins
		cmd := exec.Command("protoc",
			"--proto_path="+protoDir,
			"--go_out="+genDir,
			"--go_opt=paths=source_relative",
			"--go-grpc_out="+genDir,
			"--go-grpc_opt=paths=source_relative",
			protoFile,
		)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("protoc failed for %s: %w\nOutput: %s", protoFile, err, string(output))
		}
	}

	fmt.Println("gRPC code generation completed successfully")
	return nil
}

// checkProtocInstalled checks if protoc is installed
func checkProtocInstalled() error {
	cmd := exec.Command("protoc", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("protoc not found. Please install Protocol Buffers compiler: %w", err)
	}

	fmt.Printf("Found: %s", string(output))
	return nil
}

// checkGoProtocPlugins checks if required Go protoc plugins are installed
func checkGoProtocPlugins() error {
	plugins := []struct {
		name    string
		install string
	}{
		{"protoc-gen-go", "google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11"},
		{"protoc-gen-go-grpc", "google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1"},
	}

	for _, plugin := range plugins {
		cmd := exec.Command("which", plugin.name)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Installing %s@v1.36.11...\n", plugin.name)

			installCmd := exec.Command("go", "install", plugin.install)
			if err := installCmd.Run(); err != nil {
				return fmt.Errorf("failed to install %s: %w", plugin.name, err)
			}
			fmt.Printf("Installed %s successfully\n", plugin.name)
		} else {
			fmt.Printf("Found %s\n", plugin.name)
		}
	}

	return nil
}

// isCI checks if running in CI environment
func isCI() bool {
	return os.Getenv("CI") != "" || os.Getenv("CROUPIER_CI_BUILD") != ""
}

// findProjectRoot finds the project root directory by looking for go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check if we're in the Go SDK directory
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Check if we're in sdks/go subdirectory
			if _, err := os.Stat(filepath.Join(dir, "..", "go.mod")); err == nil {
				// Parent has go.mod, we might be in sdks/go
				if _, err := os.Stat(filepath.Join(dir, "..", "..", "proto")); err == nil {
					// Found proto/ two levels up
					return filepath.Join(dir, "../.."), nil
				}
			}
			// Check if proto exists in parent (we're in sdks/go)
			if _, err := os.Stat(filepath.Join(dir, "..", "proto")); err == nil {
				return filepath.Join(dir, ".."), nil
			}
			// Check if proto exists in current directory (we're in root)
			if _, err := os.Stat(filepath.Join(dir, "proto")); err == nil {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			return "", fmt.Errorf("could not find project root")
		}
		dir = parent
	}
}

func main() {
	fmt.Println("Croupier Go SDK Proto Generator")
	fmt.Println("===============================")

	// Check if we're in CI or local development
	isCI := isCI()
	if !isCI {
		fmt.Println("Local development build detected")
		if os.Getenv("CROUPIER_SKIP_PROTO_GEN") != "" {
			fmt.Println("CROUPIER_SKIP_PROTO_GEN is set, skipping proto generation")
			fmt.Println("Using mock gRPC implementation")
			return
		}
		fmt.Println("Tip: Set CROUPIER_SKIP_PROTO_GEN=1 to skip proto generation and use mock implementation")
	} else {
		fmt.Println("CI build detected, enabling proto generation...")
	}

	// Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		log.Fatalf("Failed to find project root: %v", err)
	}

	fmt.Printf("Project root: %s\n", projectRoot)

	// Check dependencies
	fmt.Println("\nChecking dependencies...")
	if err := checkProtocInstalled(); err != nil {
		if !isCI {
			fmt.Printf("⚠️  protoc not found: %v\n", err)
			fmt.Println("Please install protoc:")
			fmt.Println("  macOS: brew install protobuf")
			fmt.Println("  Ubuntu: sudo apt-get install protobuf-compiler")
			fmt.Println("  Or download from: https://github.com/protocolbuffers/protobuf/releases")
			fmt.Println("\nSkipping proto generation, using mock implementation")
			return
		}
		log.Fatalf("Dependency check failed: %v", err)
	}

	if err := checkGoProtocPlugins(); err != nil {
		if !isCI {
			fmt.Printf("⚠️  Go protoc plugins not found: %v\n", err)
			fmt.Println("Please install go plugins (using fixed versions):")
			fmt.Println("  go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11")
			fmt.Println("  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1")
			fmt.Println("  Make sure $GOPATH/bin is in your $PATH")
			fmt.Println("\nSkipping proto generation, using mock implementation")
			return
		}
		log.Fatalf("Go protoc plugin check failed: %v", err)
	}

	// Directories
	protoDir := filepath.Join(projectRoot, "proto")
	genDir := filepath.Join(projectRoot, "sdks/go/pkg/pb")

	// Check if proto directory exists
	if _, err := os.Stat(protoDir); os.IsNotExist(err) {
		log.Fatalf("Proto directory not found: %s", protoDir)
	}

	// Generate gRPC code
	fmt.Println("\nGenerating gRPC code...")
	if err := generateGRPCCode(protoDir, genDir); err != nil {
		log.Fatalf("Failed to generate gRPC code: %v", err)
	}

	// Create build tag file to indicate real gRPC is available
	buildTagFile := filepath.Join(projectRoot, "sdks/go/proto/build_tags.go")
	buildTagContent := `//go:build croupier_real_grpc
// +build croupier_real_grpc

package proto

// This file is generated when real gRPC proto files are available
const RealGRPCAvailable = true
`

	if err := os.WriteFile(buildTagFile, []byte(buildTagContent), 0644); err != nil {
		log.Printf("Warning: failed to create build tag file: %v", err)
	}

	fmt.Println("\n✅ Proto generation completed successfully!")
	fmt.Println("Real gRPC implementation is now available")
	fmt.Println("Run 'go build ./...' with the tag croupier_real_grpc to use it")
}
