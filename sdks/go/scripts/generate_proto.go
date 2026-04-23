package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProtoFile represents a proto file to download
type ProtoFile struct {
	Path string
	URL  string
}

// getProtoFiles returns the list of proto files to download
func getProtoFiles(branch string) []ProtoFile {
	baseURL := fmt.Sprintf("https://raw.githubusercontent.com/cuihairu/croupier/%s/proto", branch)

	return []ProtoFile{
		{
			Path: "croupier/common/v1/ui.proto",
			URL:  fmt.Sprintf("%s/croupier/common/v1/ui.proto", baseURL),
		},
		{
			Path: "croupier/agent/local/v1/local.proto",
			URL:  fmt.Sprintf("%s/croupier/agent/local/v1/local.proto", baseURL),
		},
		{
			Path: "croupier/server/v1/server_control.proto",
			URL:  fmt.Sprintf("%s/croupier/server/v1/server_control.proto", baseURL),
		},
		{
			Path: "croupier/sdk/v1/invoker.proto",
			URL:  fmt.Sprintf("%s/croupier/sdk/v1/invoker.proto", baseURL),
		},
		{
			Path: "croupier/edge/job/v1/job.proto",
			URL:  fmt.Sprintf("%s/croupier/edge/job/v1/job.proto", baseURL),
		},
		{
			Path: "croupier/server/v1/server_tunnel.proto",
			URL:  fmt.Sprintf("%s/croupier/server/v1/server_tunnel.proto", baseURL),
		},
		{
			Path: "croupier/options/v1/ui_options.proto",
			URL:  fmt.Sprintf("%s/croupier/options/v1/ui_options.proto", baseURL),
		},
		{
			Path: "croupier/options/v1/function_options.proto",
			URL:  fmt.Sprintf("%s/croupier/options/v1/function_options.proto", baseURL),
		},
	}
}

// downloadFile downloads a file from a URL
func downloadFile(url, destPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Download file
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d for %s", resp.StatusCode, url)
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", destPath, err)
	}
	defer out.Close()

	// Copy content
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", destPath, err)
	}

	return nil
}

// downloadProtoFiles downloads all proto files
func downloadProtoFiles(protoDir, branch string) error {
	files := getProtoFiles(branch)

	fmt.Printf("Downloading %d proto files to %s...\n", len(files), protoDir)

	for _, file := range files {
		destPath := filepath.Join(protoDir, file.Path)
		fmt.Printf("Downloading: %s\n", file.URL)

		if err := downloadFile(file.URL, destPath); err != nil {
			return fmt.Errorf("failed to download %s: %w", file.Path, err)
		}

		fmt.Printf("Downloaded: %s\n", file.Path)
	}

	fmt.Println("Proto files downloaded successfully")
	return nil
}

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

		// Generate protobuf code
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
	plugins := []string{
		"protoc-gen-go",
		"protoc-gen-go-grpc",
	}

	for _, plugin := range plugins {
		cmd := exec.Command("which", plugin)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Installing %s...\n", plugin)

			var installCmd *exec.Cmd
			switch plugin {
			case "protoc-gen-go":
				installCmd = exec.Command("go", "install", "google.golang.org/protobuf/cmd/protoc-gen-go@latest")
			case "protoc-gen-go-grpc":
				installCmd = exec.Command("go", "install", "google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest")
			}

			if installCmd != nil {
				if err := installCmd.Run(); err != nil {
					return fmt.Errorf("failed to install %s: %w", plugin, err)
				}
				fmt.Printf("Installed %s successfully\n", plugin)
			}
		} else {
			fmt.Printf("Found %s\n", plugin)
		}
	}

	return nil
}

// isCI checks if running in CI environment
func isCI() bool {
	return os.Getenv("CI") != "" || os.Getenv("CROUPIER_CI_BUILD") != ""
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
			fmt.Println("Please install go plugins:")
			fmt.Println("  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest")
			fmt.Println("  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest")
			fmt.Println("  Make sure $GOPATH/bin is in your $PATH")
			fmt.Println("\nSkipping proto generation, using mock implementation")
			return
		}
		log.Fatalf("Go protoc plugin check failed: %v", err)
	}

	// Get branch from environment or default to main
	branch := os.Getenv("CROUPIER_PROTO_BRANCH")
	if branch == "" {
		branch = "main"
	}

	// Directories
	protoDir := "downloaded_proto"
	genDir := "proto"

	// Download proto files
	fmt.Printf("\nDownloading proto files from branch '%s'...\n", branch)
	if err := downloadProtoFiles(protoDir, branch); err != nil {
		if !isCI {
			fmt.Printf("⚠️  Failed to download proto files: %v\n", err)
			fmt.Println("This might be due to network issues or an invalid branch name")
			fmt.Println("Skipping proto generation, using mock implementation")
			return
		}
		log.Fatalf("Failed to download proto files: %v", err)
	}

	// Generate gRPC code
	fmt.Println("\nGenerating gRPC code...")
	if err := generateGRPCCode(protoDir, genDir); err != nil {
		log.Fatalf("Failed to generate gRPC code: %v", err)
	}

	// Create build tag file to indicate real gRPC is available
	buildTagFile := "proto/build_tags.go"
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
