package graph

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// Builder builds the code graph from SSA and call graph
type Builder struct {
	fset          *token.FileSet
	pkgs          []*packages.Package
	projectRoot   string            // project root directory for relative paths
	projectPkgs   map[string]bool   // project package paths (to filter out dependencies)
	targetPkgs    map[string]bool   // target packages to insert (nil means all)
	nodeMap       map[string]int64  // maps function name to node ID
	closureParent map[string]string // maps closure name to parent function name
	insertFn      func(*Node) (int64, error)
	edgeFn        func(*Edge) error
}

// NewBuilder creates a new graph builder
func NewBuilder(
	fset *token.FileSet,
	pkgs []*packages.Package,
	projectRoot string,
	insertFn func(*Node) (int64, error),
	edgeFn func(*Edge) error,
) *Builder {
	// Extract project package paths
	projectPkgs := make(map[string]bool)
	for _, pkg := range pkgs {
		if pkg.PkgPath != "" {
			projectPkgs[pkg.PkgPath] = true
		}
	}

	// Get absolute path for projectRoot
	absRoot, _ := filepath.Abs(projectRoot)

	return &Builder{
		fset:          fset,
		pkgs:          pkgs,
		projectRoot:   absRoot,
		projectPkgs:   projectPkgs,
		targetPkgs:    nil, // nil means insert all packages
		nodeMap:       make(map[string]int64),
		closureParent: make(map[string]string),
		insertFn:      insertFn,
		edgeFn:        edgeFn,
	}
}

// SetTargetPackages sets the target packages for incremental mode
// Only functions in these packages will be inserted into the database
func (b *Builder) SetTargetPackages(pkgPaths []string) {
	if len(pkgPaths) == 0 {
		b.targetPkgs = nil
		return
	}
	b.targetPkgs = make(map[string]bool)
	for _, path := range pkgPaths {
		b.targetPkgs[path] = true
	}
}

// isProjectFunction checks if a function belongs to the project (not a dependency)
func (b *Builder) isProjectFunction(fn *ssa.Function) bool {
	if fn.Pkg == nil {
		return false
	}
	pkgPath := fn.Pkg.Pkg.Path()
	return b.projectPkgs[pkgPath]
}

// isTargetFunction checks if a function should be inserted into the database
// In incremental mode, only functions in target packages are inserted
func (b *Builder) isTargetFunction(fn *ssa.Function) bool {
	if b.targetPkgs == nil {
		// No filter, insert all project functions
		return b.isProjectFunction(fn)
	}
	if fn.Pkg == nil {
		return false
	}
	pkgPath := fn.Pkg.Pkg.Path()
	return b.targetPkgs[pkgPath]
}

// isClosure checks if a function is a closure (anonymous function)
// Closures in SSA are named with $N suffix, e.g., "indexCmd$1"
func (b *Builder) isClosure(fn *ssa.Function) bool {
	name := fn.Name()
	return strings.Contains(name, "$")
}

// getParentFunctionName extracts the parent function name from a closure name
// e.g., "github.com/foo/bar.indexCmd$1" -> "github.com/foo/bar.indexCmd"
func (b *Builder) getParentFunctionName(fn *ssa.Function) string {
	name := fn.String()
	if idx := strings.LastIndex(name, "$"); idx != -1 {
		return name[:idx]
	}
	return name
}

// resolveToParent returns the parent function name if this is a closure,
// otherwise returns the function's own name
func (b *Builder) resolveToParent(fnName string) string {
	if parent, ok := b.closureParent[fnName]; ok {
		// Recursively resolve in case of nested closures (e.g., $1$1)
		return b.resolveToParent(parent)
	}
	return fnName
}

// Build processes the call graph and stores nodes/edges
// Closures are merged into their parent functions' call chains
func (b *Builder) Build(cg *callgraph.Graph) error {
	// First pass: identify closures and map them to parent functions
	for fn := range cg.Nodes {
		if fn == nil {
			continue
		}
		if !b.isProjectFunction(fn) {
			continue
		}
		if b.isClosure(fn) {
			parentName := b.getParentFunctionName(fn)
			b.closureParent[fn.String()] = parentName
		}
	}

	// Second pass: create function nodes (skip closures)
	for fn, node := range cg.Nodes {
		if fn == nil || node == nil {
			continue
		}

		// Skip synthetic functions (init, etc.) unless they have position info
		if fn.Synthetic != "" && fn.Pos() == token.NoPos {
			continue
		}

		// Skip functions not in the project (dependencies, stdlib)
		if !b.isProjectFunction(fn) {
			continue
		}

		// Skip closures - they will be merged into parent
		if b.isClosure(fn) {
			continue
		}

		// In incremental mode, only insert functions in target packages
		if !b.isTargetFunction(fn) {
			continue
		}

		nodeID, err := b.createFunctionNode(fn)
		if err != nil {
			return fmt.Errorf("failed to create node for %s: %w", fn.String(), err)
		}
		b.nodeMap[fn.String()] = nodeID
	}

	// Third pass: create call edges (merging closure edges to parents)
	// Use a set to deduplicate edges
	edgeSet := make(map[string]bool)

	for fn, node := range cg.Nodes {
		if fn == nil || node == nil {
			continue
		}

		// Resolve caller to parent if it's a closure
		callerName := b.resolveToParent(fn.String())
		fromID, ok := b.nodeMap[callerName]
		if !ok {
			continue
		}

		for _, edge := range node.Out {
			if edge.Callee == nil || edge.Callee.Func == nil {
				continue
			}

			// Resolve callee to parent if it's a closure
			calleeName := b.resolveToParent(edge.Callee.Func.String())
			toID, ok := b.nodeMap[calleeName]
			if !ok {
				continue
			}

			// Skip self-loops that may arise from closure merging
			if fromID == toID {
				continue
			}

			// Deduplicate edges
			edgeKey := fmt.Sprintf("%d->%d", fromID, toID)
			if edgeSet[edgeKey] {
				continue
			}
			edgeSet[edgeKey] = true

			// Get call site info
			var callSiteFile string
			var callSiteLine int
			if edge.Site != nil && edge.Site.Pos() != token.NoPos {
				pos := b.fset.Position(edge.Site.Pos())
				callSiteFile = pos.Filename
				callSiteLine = pos.Line
			}

			err := b.edgeFn(&Edge{
				FromID:       fromID,
				ToID:         toID,
				Kind:         EdgeKindCalls,
				CallSiteFile: callSiteFile,
				CallSiteLine: callSiteLine,
			})
			if err != nil {
				return fmt.Errorf("failed to create edge: %w", err)
			}
		}
	}

	return nil
}

// createFunctionNode creates a node for a function
func (b *Builder) createFunctionNode(fn *ssa.Function) (int64, error) {
	pos := b.fset.Position(fn.Pos())

	// Build signature string
	sig := fn.Signature.String()

	// Get doc comment if available
	doc := b.getDocComment(fn)

	// Determine package path
	pkgPath := ""
	if fn.Pkg != nil {
		pkgPath = fn.Pkg.Pkg.Path()
	}

	// Build fully qualified name
	name := fn.String()

	// Convert file path to relative path
	filePath := pos.Filename
	if b.projectRoot != "" && filePath != "" {
		if rel, err := filepath.Rel(b.projectRoot, filePath); err == nil {
			filePath = rel
		}
	}

	node := &Node{
		Kind:      NodeKindFunc,
		Name:      name,
		Package:   pkgPath,
		File:      filePath,
		Line:      pos.Line,
		Signature: sig,
		Doc:       doc,
	}

	return b.insertFn(node)
}

// getDocComment extracts the doc comment for a function
func (b *Builder) getDocComment(fn *ssa.Function) string {
	if fn.Syntax() == nil {
		return ""
	}

	// Try to get the AST node
	switch syntax := fn.Syntax().(type) {
	case *ast.FuncDecl:
		if syntax.Doc != nil {
			return strings.TrimSpace(syntax.Doc.Text())
		}
	case *ast.FuncLit:
		// Function literals don't have doc comments
		return ""
	}

	return ""
}

// BuildStats returns statistics about the built graph
type BuildStats struct {
	TotalNodes int
	TotalEdges int
}

// GetNodeCount returns the number of nodes created
func (b *Builder) GetNodeCount() int {
	return len(b.nodeMap)
}

