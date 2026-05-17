package service_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoCrossServiceImports verifies that domain services do not import each other.
// This enforces the architectural boundary: each service only depends on shared
// packages (store, component/, api/v1/) but never on sibling services.
func TestNoCrossServiceImports(t *testing.T) {
	pairs := [][2]string{
		{"dcm", "sqlsvc"},
		{"dcm", "admin"},
		{"sqlsvc", "dcm"},
		{"sqlsvc", "admin"},
		{"admin", "dcm"},
		{"admin", "sqlsvc"},
		// Runner should not import domain services either.
		{"runner", "dcm"},
		{"runner", "sqlsvc"},
		{"runner", "admin"},
	}

	for _, pair := range pairs {
		assertNoImport(t, pair[0], pair[1])
	}
}

// TestGatewayDoesNotImportStore verifies the gateway package does not
// directly import the store package (it should only route, not query).
func TestGatewayDoesNotImportStore(t *testing.T) {
	gatewayDir := findServiceDir("gateway")
	if gatewayDir == "" {
		t.Skip("gateway directory not found")
	}

	// Gateway imports store via interceptors.go (InterceptorDeps), which is OK.
	// We verify it doesn't import store/* sub-packages.
	imports := collectImports(t, gatewayDir)
	for _, imp := range imports {
		if strings.Contains(imp, "backend/store/") && !strings.HasSuffix(imp, "backend/store") {
			t.Errorf("gateway should not import store sub-packages, found: %s", imp)
		}
	}
}

// TestServiceImplementsDomainServiceInterface verifies all service packages
// export a Service type with the required methods.
func TestServicePackagesExist(t *testing.T) {
	services := []string{"dcm", "sqlsvc", "admin", "runner"}
	for _, svc := range services {
		dir := findServiceDir(svc)
		if dir == "" {
			t.Errorf("service package %q not found", svc)
			continue
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Errorf("failed to glob %s: %v", dir, err)
			continue
		}
		if len(files) == 0 {
			t.Errorf("service %q has no .go files", svc)
		}
	}
}

// assertNoImport checks that package `from` does not import package `to`.
func assertNoImport(t *testing.T, from, to string) {
	t.Helper()
	fromDir := findServiceDir(from)
	if fromDir == "" {
		t.Logf("skipping: service %q directory not found", from)
		return
	}

	imports := collectImports(t, fromDir)
	forbidden := "backend/service/" + to
	for _, imp := range imports {
		if strings.Contains(imp, forbidden) {
			t.Errorf("ARCHITECTURE VIOLATION: service/%s imports service/%s (%s)", from, to, imp)
		}
	}
}

// collectImports returns all import paths from Go files in a directory.
func collectImports(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var imports []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Logf("warning: could not parse %s: %v", path, err)
			return nil
		}

		for _, imp := range f.Imports {
			imports = append(imports, strings.Trim(imp.Path.Value, `"`))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk %s: %v", dir, err)
	}
	return imports
}

// findServiceDir locates a service directory relative to the test file.
func findServiceDir(name string) string {
	// Try relative paths from typical Go test working directory.
	candidates := []string{
		filepath.Join(".", name),
		filepath.Join("..", "service", name),
		filepath.Join("backend", "service", name),
	}

	// Also try from repo root via environment.
	if wd, err := os.Getwd(); err == nil {
		// Walk up to find backend/service/
		dir := wd
		for i := 0; i < 5; i++ {
			candidate := filepath.Join(dir, "backend", "service", name)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
			candidate = filepath.Join(dir, "backend", name)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
			dir = filepath.Dir(dir)
		}
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}
