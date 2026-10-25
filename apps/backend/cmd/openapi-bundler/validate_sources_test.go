package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestValidateIndividualYamlFiles(t *testing.T) {
	root := filepath.Join(findRepoRoot(), "docs", "openapi")

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			return nil
		}
		var node yaml.Node
		if err := yaml.Unmarshal(data, &node); err != nil {
			t.Errorf("YAML error in %s:\n%v", path, err)
		} else {
			fmt.Printf("PASS: %s\n", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
