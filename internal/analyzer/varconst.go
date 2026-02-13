package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/zheng/crag/internal/graph"
)

// VarConstInfo represents a package-level variable or constant
type VarConstInfo struct {
	Name    string         // Full name: pkg.VarName
	Package string         // Package path
	File    string         // Source file (relative)
	Line    int            // Line number
	Kind    graph.NodeKind // var or const
	TypeStr string         // Type as string
	Doc     string         // Documentation comment
}

// ReferenceInfo represents a function referencing a var/const
type ReferenceInfo struct {
	FuncName     string // Full function name
	VarConstName string // Full var/const name
	File         string // Reference site file
	Line         int    // Reference site line
}

// VarConstAnalyzer analyzes package-level variables and constants
type VarConstAnalyzer struct {
	pkgs        []*packages.Package
	projectRoot string
	projectPkgs map[string]bool
	targetPkgs  map[string]bool // target packages for incremental mode (nil means all)
}

// NewVarConstAnalyzer creates a new var/const analyzer
func NewVarConstAnalyzer(pkgs []*packages.Package, projectRoot string) *VarConstAnalyzer {
	projectPkgs := make(map[string]bool)
	for _, pkg := range pkgs {
		if pkg.PkgPath != "" {
			projectPkgs[pkg.PkgPath] = true
		}
	}

	absRoot, _ := filepath.Abs(projectRoot)

	return &VarConstAnalyzer{
		pkgs:        pkgs,
		projectRoot: absRoot,
		projectPkgs: projectPkgs,
	}
}

// SetTargetPackages sets the target packages for incremental mode.
// Only var/const in these packages will be inserted as nodes.
func (a *VarConstAnalyzer) SetTargetPackages(pkgPaths []string) {
	if len(pkgPaths) == 0 {
		a.targetPkgs = nil
		return
	}
	a.targetPkgs = make(map[string]bool)
	for _, path := range pkgPaths {
		a.targetPkgs[path] = true
	}
}

// isTargetPackage checks if a package should have its var/const nodes inserted
func (a *VarConstAnalyzer) isTargetPackage(pkgPath string) bool {
	if a.targetPkgs == nil {
		return true
	}
	return a.targetPkgs[pkgPath]
}

// Analyze collects all package-level vars and consts
func (a *VarConstAnalyzer) Analyze() []*VarConstInfo {
	var results []*VarConstInfo

	for _, pkg := range a.pkgs {
		if pkg.Types == nil {
			continue
		}
		if !a.projectPkgs[pkg.PkgPath] {
			continue
		}
		if !a.isTargetPackage(pkg.PkgPath) {
			continue
		}

		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}

			// Skip unexported names
			if !obj.Exported() {
				continue
			}

			pos := pkg.Fset.Position(obj.Pos())
			file := pos.Filename
			if a.projectRoot != "" && file != "" {
				if rel, err := filepath.Rel(a.projectRoot, file); err == nil {
					file = rel
				}
			}

			switch o := obj.(type) {
			case *types.Var:
				results = append(results, &VarConstInfo{
					Name:    pkg.PkgPath + "." + name,
					Package: pkg.PkgPath,
					File:    file,
					Line:    pos.Line,
					Kind:    graph.NodeKindVar,
					TypeStr: o.Type().String(),
					Doc:     a.getVarConstDoc(pkg, name),
				})
			case *types.Const:
				results = append(results, &VarConstInfo{
					Name:    pkg.PkgPath + "." + name,
					Package: pkg.PkgPath,
					File:    file,
					Line:    pos.Line,
					Kind:    graph.NodeKindConst,
					TypeStr: o.Type().String(),
					Doc:     a.getVarConstDoc(pkg, name),
				})
			}
		}
	}
	return results
}

// FindReferences finds which functions reference each var/const
func (a *VarConstAnalyzer) FindReferences(varConsts []*VarConstInfo) []*ReferenceInfo {
	// Build a set of var/const objects for lookup
	varConstObjs := make(map[types.Object]string) // obj -> full name

	for _, pkg := range a.pkgs {
		if pkg.Types == nil || !a.projectPkgs[pkg.PkgPath] {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil || !obj.Exported() {
				continue
			}
			switch obj.(type) {
			case *types.Var, *types.Const:
				fullName := pkg.PkgPath + "." + name
				varConstObjs[obj] = fullName
			}
		}
	}

	if len(varConstObjs) == 0 {
		return nil
	}

	var refs []*ReferenceInfo
	refSet := make(map[string]bool) // dedup: "funcName->varConstName"

	for _, pkg := range a.pkgs {
		if pkg.TypesInfo == nil || !a.projectPkgs[pkg.PkgPath] {
			continue
		}

		for _, astFile := range pkg.Syntax {
			for _, decl := range astFile.Decls {
				funcDecl, ok := decl.(*ast.FuncDecl)
				if !ok || funcDecl.Body == nil {
					continue
				}
				funcName := a.getFuncFullName(pkg, funcDecl)
				a.walkFuncBody(pkg, funcDecl.Body, funcName, varConstObjs, refSet, &refs)
			}
		}
	}

	return refs
}

// walkFuncBody walks a function body looking for var/const references
func (a *VarConstAnalyzer) walkFuncBody(
	pkg *packages.Package,
	body *ast.BlockStmt,
	funcName string,
	varConstObjs map[types.Object]string,
	refSet map[string]bool,
	refs *[]*ReferenceInfo,
) {
	ast.Inspect(body, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		// Look up what this identifier refers to
		obj := pkg.TypesInfo.Uses[ident]
		if obj == nil {
			return true
		}

		vcName, found := varConstObjs[obj]
		if !found {
			return true
		}

		key := funcName + "->" + vcName
		if refSet[key] {
			return true
		}
		refSet[key] = true

		pos := pkg.Fset.Position(ident.Pos())
		file := pos.Filename
		if a.projectRoot != "" && file != "" {
			if rel, err := filepath.Rel(a.projectRoot, file); err == nil {
				file = rel
			}
		}

		*refs = append(*refs, &ReferenceInfo{
			FuncName:     funcName,
			VarConstName: vcName,
			File:         file,
			Line:         pos.Line,
		})

		return true
	})
}

// getFuncFullName builds the fully qualified function name
func (a *VarConstAnalyzer) getFuncFullName(pkg *packages.Package, funcDecl *ast.FuncDecl) string {
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		recv := funcDecl.Recv.List[0]
		typExpr := recv.Type
		// Unwrap pointer receiver
		if star, ok := typExpr.(*ast.StarExpr); ok {
			typExpr = star.X
		}
		if ident, ok := typExpr.(*ast.Ident); ok {
			return "(" + pkg.PkgPath + "." + ident.Name + ")." + funcDecl.Name.Name
		}
	}
	return pkg.PkgPath + "." + funcDecl.Name.Name
}

// getVarConstDoc extracts doc comment for a var/const declaration
func (a *VarConstAnalyzer) getVarConstDoc(pkg *packages.Package, name string) string {
	for _, astFile := range pkg.Syntax {
		for _, decl := range astFile.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			if genDecl.Tok != token.VAR && genDecl.Tok != token.CONST {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, ident := range valueSpec.Names {
					if ident.Name == name {
						if valueSpec.Doc != nil {
							return strings.TrimSpace(valueSpec.Doc.Text())
						}
						if genDecl.Doc != nil {
							return strings.TrimSpace(genDecl.Doc.Text())
						}
						return ""
					}
				}
			}
		}
	}
	return ""
}

// BuildVarConstGraph builds the var/const reference graph
func (a *VarConstAnalyzer) BuildVarConstGraph(
	insertNodeFn func(*graph.Node) (int64, error),
	insertEdgeFn func(*graph.Edge) error,
	existingNodeMap map[string]int64,
) (varCount, constCount, refCount int, err error) {
	varConsts := a.Analyze()

	if len(varConsts) == 0 {
		return 0, 0, 0, nil
	}

	// Insert var/const nodes
	vcNodeIDs := make(map[string]int64)
	for _, vc := range varConsts {
		node := &graph.Node{
			Kind:      vc.Kind,
			Name:      vc.Name,
			Package:   vc.Package,
			File:      vc.File,
			Line:      vc.Line,
			Signature: vc.TypeStr,
			Doc:       vc.Doc,
		}
		id, err := insertNodeFn(node)
		if err != nil {
			return 0, 0, 0, err
		}
		vcNodeIDs[vc.Name] = id
		if vc.Kind == graph.NodeKindVar {
			varCount++
		} else {
			constCount++
		}
	}

	// Find and insert references
	refs := a.FindReferences(varConsts)
	for _, ref := range refs {
		funcID, funcOK := existingNodeMap[ref.FuncName]
		vcID, vcOK := vcNodeIDs[ref.VarConstName]
		if !funcOK || !vcOK {
			continue
		}

		edge := &graph.Edge{
			FromID:       funcID,
			ToID:         vcID,
			Kind:         graph.EdgeKindReferences,
			CallSiteFile: ref.File,
			CallSiteLine: ref.Line,
		}
		if err := insertEdgeFn(edge); err != nil {
			return 0, 0, 0, err
		}
		refCount++
	}

	return varCount, constCount, refCount, nil
}
