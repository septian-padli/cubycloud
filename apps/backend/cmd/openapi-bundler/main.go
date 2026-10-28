package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type BundleOptions struct {
	SourceDir   string
	OutputFile  string
	BackendCopy string
	CheckOnly   bool
}

func main() {
	sourceDir := flag.String("source", "docs/openapi", "Directory containing root.yaml, paths/, and components/")
	outputFile := flag.String("output", "docs/openapi.yaml", "Output path for bundled OpenAPI spec")
	backendCopy := flag.String("backend-copy", "apps/backend/internal/modules/docs/openapi.yaml", "Path to copy bundled spec for Go embedding")
	checkOnly := flag.Bool("check", false, "Verify bundling without writing changes")
	flag.Parse()

	opts := BundleOptions{
		SourceDir:   *sourceDir,
		OutputFile:  *outputFile,
		BackendCopy: *backendCopy,
		CheckOnly:   *checkOnly,
	}

	if err := Run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func Run(opts BundleOptions) error {
	bundledBytes, err := BundleToBytes(opts.SourceDir)
	if err != nil {
		return err
	}

	if opts.CheckOnly {
		existingBytes, err := os.ReadFile(opts.OutputFile)
		if err != nil {
			return fmt.Errorf("failed to read existing output file for check: %w", err)
		}
		if string(existingBytes) != string(bundledBytes) {
			return fmt.Errorf("bundled file %s is stale; run 'go run ./cmd/openapi-bundler' to regenerate", opts.OutputFile)
		}
		if opts.BackendCopy != "" {
			backendBytes, err := os.ReadFile(opts.BackendCopy)
			if err != nil || string(backendBytes) != string(bundledBytes) {
				return fmt.Errorf("backend copy %s is stale or missing; run 'go run ./cmd/openapi-bundler' to sync", opts.BackendCopy)
			}
		}
		fmt.Println("OpenAPI bundle is up to date.")
		return nil
	}

	if err := os.WriteFile(opts.OutputFile, bundledBytes, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", opts.OutputFile, err)
	}
	fmt.Printf("Successfully bundled OpenAPI specification to %s (%d bytes)\n", opts.OutputFile, len(bundledBytes))

	if opts.BackendCopy != "" {
		if err := os.MkdirAll(filepath.Dir(opts.BackendCopy), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", opts.BackendCopy, err)
		}
		if err := os.WriteFile(opts.BackendCopy, bundledBytes, 0644); err != nil {
			return fmt.Errorf("failed to write backend copy %s: %w", opts.BackendCopy, err)
		}
		fmt.Printf("Synced OpenAPI specification to backend embed: %s\n", opts.BackendCopy)
	}

	return nil
}

func BundleToBytes(sourceDir string) ([]byte, error) {
	rootPath := filepath.Join(sourceDir, "root.yaml")
	rootBytes, err := os.ReadFile(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read root.yaml: %w", err)
	}

	var rootNode yaml.Node
	if err := yaml.Unmarshal(rootBytes, &rootNode); err != nil {
		return nil, fmt.Errorf("failed to parse root.yaml: %w", err)
	}

	// Paths map
	pathsMap := make(map[string]any)
	pathsDir := filepath.Join(sourceDir, "paths")
	pathsEntries, err := os.ReadDir(pathsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read paths directory: %w", err)
	}

	// Sort file names for deterministic order
	var pathFiles []string
	for _, entry := range pathsEntries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
			pathFiles = append(pathFiles, entry.Name())
		}
	}
	sort.Strings(pathFiles)

	for _, fileName := range pathFiles {
		filePath := filepath.Join(pathsDir, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		var filePaths map[string]any
		if err := yaml.Unmarshal(content, &filePaths); err != nil {
			return nil, fmt.Errorf("YAML syntax error in %s: %w", filePath, err)
		}

		for pathKey, pathValue := range filePaths {
			if existing, exists := pathsMap[pathKey]; exists {
				existingMap, ok1 := existing.(map[string]any)
				valMap, ok2 := pathValue.(map[string]any)
				if ok1 && ok2 {
					for method, methodVal := range valMap {
						if _, duplicateMethod := existingMap[method]; duplicateMethod {
							return nil, fmt.Errorf("duplicate method %s for path %s in %s", method, pathKey, filePath)
						}
						existingMap[method] = methodVal
					}
					pathsMap[pathKey] = existingMap
					continue
				}
				return nil, fmt.Errorf("duplicate path key %s across files (in %s)", pathKey, filePath)
			}
			pathsMap[pathKey] = pathValue
		}
	}

	// Schemas map
	componentsSchemas := make(map[string]any)
	componentsDir := filepath.Join(sourceDir, "components")
	compEntries, err := os.ReadDir(componentsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read components directory: %w", err)
	}

	var compFiles []string
	for _, entry := range compEntries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
			compFiles = append(compFiles, entry.Name())
		}
	}
	sort.Strings(compFiles)

	for _, fileName := range compFiles {
		filePath := filepath.Join(componentsDir, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		var fileSchemas map[string]any
		if err := yaml.Unmarshal(content, &fileSchemas); err != nil {
			return nil, fmt.Errorf("YAML syntax error in %s: %w", filePath, err)
		}

		for schemaName, schemaVal := range fileSchemas {
			if _, exists := componentsSchemas[schemaName]; exists {
				return nil, fmt.Errorf("duplicate schema %s found in %s", schemaName, filePath)
			}
			componentsSchemas[schemaName] = schemaVal
		}
	}

	// Unmarshal root into generic map to merge
	var rootDoc map[string]any
	if err := yaml.Unmarshal(rootBytes, &rootDoc); err != nil {
		return nil, fmt.Errorf("failed to parse rootDoc: %w", err)
	}

	rootDoc["paths"] = pathsMap

	componentsAny, ok := rootDoc["components"]
	var components map[string]any
	if !ok || componentsAny == nil {
		components = make(map[string]any)
		rootDoc["components"] = components
	} else {
		components, ok = componentsAny.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("components in root.yaml is not a map")
		}
	}
	components["schemas"] = componentsSchemas

	return yaml.Marshal(rootDoc)
}
