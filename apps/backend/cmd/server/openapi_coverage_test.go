package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/cubycloud/backend/internal/config"
	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	jobsmod "github.com/cubycloud/backend/internal/modules/jobs"
	"github.com/cubycloud/backend/internal/sse"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func findRepoRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "docs", "openapi")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}

type OpenAPISpec struct {
	OpenAPI    string                    `yaml:"openapi"`
	Info       map[string]any            `yaml:"info"`
	Paths      map[string]map[string]any `yaml:"paths"`
	Components struct {
		Schemas         map[string]any `yaml:"schemas"`
		SecuritySchemes map[string]any `yaml:"securitySchemes"`
	} `yaml:"components"`
}

type OperationDoc struct {
	OperationID string         `yaml:"operationId"`
	Summary     string         `yaml:"summary"`
	Description string         `yaml:"description"`
	Tags        []string       `yaml:"tags"`
	Responses   map[string]any `yaml:"responses"`
}

func normalizeEchoPath(p string) string {
	// Replace :param with {param}
	re := regexp.MustCompile(`:([a-zA-Z0-9_]+)`)
	normalized := re.ReplaceAllString(p, `{$1}`)
	// Normalize /api/docs/ to /api/docs
	if normalized == "/api/docs/" {
		normalized = "/api/docs"
	}
	return normalized
}

func TestOpenAPIRouteCoverageAndValidation(t *testing.T) {
	repoRoot := findRepoRoot()
	yamlPath := filepath.Join(repoRoot, "docs", "openapi.yaml")

	yamlBytes, err := os.ReadFile(yamlPath)
	require.NoError(t, err, "docs/openapi.yaml must exist")

	var spec OpenAPISpec
	err = yaml.Unmarshal(yamlBytes, &spec)
	require.NoError(t, err, "docs/openapi.yaml must be valid YAML")

	// 1. Validate OpenAPI version is 3.1
	assert.True(t, strings.HasPrefix(spec.OpenAPI, "3.1"), "OpenAPI specification must be version 3.1.x, got %s", spec.OpenAPI)

	// 2. Gather all OpenAPI operations
	documentedOps := make(map[string]string) // "METHOD /path" -> operationId
	opIDs := make(map[string]string)        // operationId -> "METHOD /path"

	for pathKey, methods := range spec.Paths {
		for methodKey, opAny := range methods {
			if strings.HasPrefix(methodKey, "x-") || methodKey == "parameters" {
				continue
			}
			opBytes, err := yaml.Marshal(opAny)
			require.NoError(t, err)
			var op OperationDoc
			err = yaml.Unmarshal(opBytes, &op)
			require.NoError(t, err)

			httpMethod := strings.ToUpper(methodKey)
			key := fmt.Sprintf("%s %s", httpMethod, pathKey)

			require.NotEmpty(t, op.OperationID, "Operation %s must have a non-empty operationId", key)
			require.NotEmpty(t, op.Summary, "Operation %s must have a non-empty summary", key)
			require.NotEmpty(t, op.Responses, "Operation %s must define responses", key)

			if prev, exists := opIDs[op.OperationID]; exists {
				t.Errorf("Duplicate operationId '%s' found in '%s' and '%s'", op.OperationID, prev, key)
			}
			opIDs[op.OperationID] = key
			documentedOps[key] = op.OperationID
		}
	}

	// 3. Collect registered backend routes from Echo
	tempDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "db"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "storage"), 0755))

	db, err := database.Connect(tempDir)
	require.NoError(t, err)
	defer db.Close()

	driver := storage.NewStorageDriver(db, filepath.Join(tempDir, "storage"))
	cfg := &config.Config{
		Port:           "8383",
		DataDirPath:    tempDir,
		DBPath:         filepath.Join(tempDir, "db", "cubycloud.db"),
		StoragePath:    filepath.Join(tempDir, "storage"),
		JWTSecret:      "test-secret-at-least-32-bytes-long-super-secret",
		AllowedOrigins: []string{"*"},
	}
	setupGuard := middleware.NewSetupGuard(true)
	sseHub := sse.NewHub()
	jobQueue := jobsmod.NewQueue(db)

	e := buildEchoRouter(cfg, db, driver, setupGuard, sseHub, jobQueue)

	registeredOps := make(map[string]bool)
	for _, r := range e.Routes() {
		// Ignore internal Echo routing placeholders
		if r.Method == "echo_route_not_found" {
			continue
		}
		// Ignore static documentation assets and SPA catchall
		if r.Path == "/*" || r.Path == "/api/docs/assets/*" {
			continue
		}

		normPath := normalizeEchoPath(r.Path)
		key := fmt.Sprintf("%s %s", r.Method, normPath)
		registeredOps[key] = true
	}

	// 4. Check for undocumented backend routes
	var undocumented []string
	for regKey := range registeredOps {
		if _, exists := documentedOps[regKey]; !exists {
			undocumented = append(undocumented, regKey)
		}
	}
	sort.Strings(undocumented)
	assert.Empty(t, undocumented, "Discovered %d undocumented backend route(s):\n%s", len(undocumented), strings.Join(undocumented, "\n"))

	// 5. Check for documented operations that do not exist in backend routes
	var phantomOps []string
	for docKey := range documentedOps {
		if !registeredOps[docKey] {
			phantomOps = append(phantomOps, docKey)
		}
	}
	sort.Strings(phantomOps)
	assert.Empty(t, phantomOps, "Discovered %d documented operation(s) not registered in Echo router:\n%s", len(phantomOps), strings.Join(phantomOps, "\n"))

	// 6. Verify all $ref pointers in YAML point to real schemas
	refPattern := regexp.MustCompile(`['"]?\$ref['"]?:\s*['"]?#/components/schemas/([a-zA-Z0-9_]+)['"]?`)
	matches := refPattern.FindAllStringSubmatch(string(yamlBytes), -1)
	unresolvedRefs := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			schemaName := match[1]
			if _, exists := spec.Components.Schemas[schemaName]; !exists {
				unresolvedRefs[schemaName] = true
			}
		}
	}

	var missingSchemas []string
	for schema := range unresolvedRefs {
		missingSchemas = append(missingSchemas, schema)
	}
	sort.Strings(missingSchemas)
	assert.Empty(t, missingSchemas, "Unresolved schema references:\n%s", strings.Join(missingSchemas, "\n"))

	// 7. Check that backend embedded copy matches root docs/openapi.yaml
	backendYamlPath := filepath.Join(repoRoot, "apps", "backend", "internal", "modules", "docs", "openapi.yaml")
	backendBytes, err := os.ReadFile(backendYamlPath)
	require.NoError(t, err)
	assert.Equal(t, string(yamlBytes), string(backendBytes), "apps/backend/internal/modules/docs/openapi.yaml must be identical to docs/openapi.yaml")

	fmt.Printf("\n=== OPENAPI COVERAGE VERIFICATION: %d/%d OPERATIONS COVERED (100%%) ===\n", len(documentedOps), len(registeredOps))
}

func TestOpenAPISpecFreshness(t *testing.T) {
	repoRoot := findRepoRoot()
	cmd := exec.Command("go", "run", "./cmd/openapi-bundler",
		"-source", filepath.Join(repoRoot, "docs", "openapi"),
		"-output", filepath.Join(repoRoot, "docs", "openapi.yaml"),
		"-backend-copy", filepath.Join(repoRoot, "apps", "backend", "internal", "modules", "docs", "openapi.yaml"),
		"-check",
	)
	cmd.Dir = filepath.Join(repoRoot, "apps", "backend")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "docs/openapi.yaml or backend copy is stale! Run 'go run ./cmd/openapi-bundler' to regenerate:\n%s", string(out))
}
