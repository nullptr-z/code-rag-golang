package analyzer

import (
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// BuildSSA builds the SSA representation for the given packages
func BuildSSA(pkgs []*packages.Package) (*ssa.Program, []*ssa.Package) {
	// Create SSA program with generics support
	prog, ssaPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)

	// Build SSA for all packages
	prog.Build()

	return prog, ssaPkgs
}
