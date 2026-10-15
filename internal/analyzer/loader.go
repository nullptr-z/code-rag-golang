package analyzer

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

// IsGoProject reports whether projectPath looks like a Go project, i.e. it
// (or one of its immediate subdirectories) contains a go.mod file. crag only
// understands Go, so callers must check this before opening/creating a
// .crag.db — otherwise every non-Go project ends up with a useless empty
// database file just from being pointed at by `crag mcp`.
func IsGoProject(projectPath string) bool {
	if _, err := os.Stat(filepath.Join(projectPath, "go.mod")); err == nil {
		return true
	}

	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "vendor" || name == "node_modules" || name[0] == '.' {
			continue
		}
		if _, err := os.Stat(filepath.Join(projectPath, name, "go.mod")); err == nil {
			return true
		}
	}
	return false
}

// LoadPackages loads all Go packages from the given project path
func LoadPackages(projectPath string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps |
			packages.NeedImports,
		Dir: projectPath,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("failed to load packages: %w", err)
	}

	// Check for errors in loaded packages
	var errs []error
	for _, pkg := range pkgs {
		for _, err := range pkg.Errors {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		// Log errors but continue - some packages may still be usable
		fmt.Printf("Warning: %d package errors encountered\n", len(errs))
		for _, err := range errs {
			fmt.Printf("  - %v\n", err)
		}
	}

	return pkgs, nil
}

// FilterMainPackages filters packages to only include those with source files
func FilterMainPackages(pkgs []*packages.Package) []*packages.Package {
	var result []*packages.Package
	for _, pkg := range pkgs {
		if len(pkg.Syntax) > 0 {
			result = append(result, pkg)
		}
	}
	return result
}

