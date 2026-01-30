package analyzer

import (
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/packages"

	"github.com/zheng/crag/internal/graph"
)

// InterfaceInfo represents an interface definition
type InterfaceInfo struct {
	Name       string   // Full name: pkg.InterfaceName
	Package    string   // Package path
	File       string   // Source file
	Line       int      // Line number
	Methods    []string // Method signatures
	MethodsStr string   // Methods as string for display
}

// TypeInfo represents a named type (struct, etc.)
type TypeInfo struct {
	Name    string // Full name: pkg.TypeName
	Package string // Package path
	File    string // Source file
	Line    int    // Line number
}

// Implementation represents a type implementing an interface
type Implementation struct {
	Type      *TypeInfo
	Interface *InterfaceInfo
	IsPointer bool // Whether *T implements I (vs T implements I)
}

// InterfaceAnalyzer analyzes interface implementations
type InterfaceAnalyzer struct {
	pkgs        []*packages.Package
	projectRoot string
	projectPkgs map[string]bool
}

// NewInterfaceAnalyzer creates a new interface analyzer
func NewInterfaceAnalyzer(pkgs []*packages.Package, projectRoot string) *InterfaceAnalyzer {
	projectPkgs := make(map[string]bool)
	for _, pkg := range pkgs {
		if pkg.PkgPath != "" {
			projectPkgs[pkg.PkgPath] = true
		}
	}

	absRoot, _ := filepath.Abs(projectRoot)

	return &InterfaceAnalyzer{
		pkgs:        pkgs,
		projectRoot: absRoot,
		projectPkgs: projectPkgs,
	}
}

// Analyze extracts interfaces, types, and their implementation relationships
func (a *InterfaceAnalyzer) Analyze() (interfaces []*InterfaceInfo, typInfos []*TypeInfo, impls []*Implementation) {
	// Collect all interfaces and types from the project
	for _, pkg := range a.pkgs {
		if pkg.Types == nil {
			continue
		}

		// Only analyze project packages
		if !a.projectPkgs[pkg.PkgPath] {
			continue
		}

		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}

			// Get position info
			pos := pkg.Fset.Position(obj.Pos())
			file := pos.Filename
			if a.projectRoot != "" && file != "" {
				if rel, err := filepath.Rel(a.projectRoot, file); err == nil {
					file = rel
				}
			}

			typeName, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}

			named, ok := typeName.Type().(*types.Named)
			if !ok {
				continue
			}

			underlying := named.Underlying()

			// Check if it's an interface
			if iface, ok := underlying.(*types.Interface); ok {
				methods := make([]string, iface.NumMethods())
				for i := 0; i < iface.NumMethods(); i++ {
					m := iface.Method(i)
					methods[i] = m.Name() + m.Type().(*types.Signature).String()[4:] // Remove "func" prefix
				}

				interfaces = append(interfaces, &InterfaceInfo{
					Name:       pkg.PkgPath + "." + name,
					Package:    pkg.PkgPath,
					File:       file,
					Line:       pos.Line,
					Methods:    methods,
					MethodsStr: formatMethods(methods),
				})
			} else {
				// It's a named type (struct, etc.)
				typInfos = append(typInfos, &TypeInfo{
					Name:    pkg.PkgPath + "." + name,
					Package: pkg.PkgPath,
					File:    file,
					Line:    pos.Line,
				})
			}
		}
	}

	// Find implementation relationships
	for _, iface := range interfaces {
		ifaceType := a.findInterface(iface.Name)
		if ifaceType == nil {
			continue
		}

		for _, typ := range typInfos {
			namedType := a.findNamedType(typ.Name)
			if namedType == nil {
				continue
			}

			// Check if T implements I
			if types.Implements(namedType, ifaceType) {
				impls = append(impls, &Implementation{
					Type:      typ,
					Interface: iface,
					IsPointer: false,
				})
			} else if types.Implements(types.NewPointer(namedType), ifaceType) {
				// Check if *T implements I
				impls = append(impls, &Implementation{
					Type:      typ,
					Interface: iface,
					IsPointer: true,
				})
			}
		}
	}

	return
}

// findInterface finds an interface type by full name
func (a *InterfaceAnalyzer) findInterface(fullName string) *types.Interface {
	for _, pkg := range a.pkgs {
		if pkg.Types == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}
			typeName, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}
			if pkg.PkgPath+"."+name == fullName {
				if named, ok := typeName.Type().(*types.Named); ok {
					if iface, ok := named.Underlying().(*types.Interface); ok {
						return iface
					}
				}
			}
		}
	}
	return nil
}

// findNamedType finds a named type by full name
func (a *InterfaceAnalyzer) findNamedType(fullName string) *types.Named {
	for _, pkg := range a.pkgs {
		if pkg.Types == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}
			typeName, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}
			if pkg.PkgPath+"."+name == fullName {
				if named, ok := typeName.Type().(*types.Named); ok {
					return named
				}
			}
		}
	}
	return nil
}

// formatMethods formats method list for display
func formatMethods(methods []string) string {
	if len(methods) == 0 {
		return "(empty interface)"
	}
	result := ""
	for i, m := range methods {
		if i > 0 {
			result += ", "
		}
		result += m
	}
	return result
}

// BuildInterfaceGraph builds the interface implementation graph and returns insertable data
func (a *InterfaceAnalyzer) BuildInterfaceGraph(
	insertNodeFn func(*graph.Node) (int64, error),
	insertEdgeFn func(*graph.Edge) error,
) (interfaceCount, typeCount, implCount int, err error) {
	interfaces, typInfos, impls := a.Analyze()

	// Maps for tracking node IDs
	interfaceIDs := make(map[string]int64)
	typeIDs := make(map[string]int64)

	// Insert interfaces as nodes
	for _, iface := range interfaces {
		node := &graph.Node{
			Kind:      graph.NodeKindInterface,
			Name:      iface.Name,
			Package:   iface.Package,
			File:      iface.File,
			Line:      iface.Line,
			Signature: iface.MethodsStr,
		}
		id, err := insertNodeFn(node)
		if err != nil {
			return 0, 0, 0, err
		}
		interfaceIDs[iface.Name] = id
		interfaceCount++
	}

	// Insert types as nodes
	for _, typ := range typInfos {
		node := &graph.Node{
			Kind:    graph.NodeKindStruct,
			Name:    typ.Name,
			Package: typ.Package,
			File:    typ.File,
			Line:    typ.Line,
		}
		id, err := insertNodeFn(node)
		if err != nil {
			return 0, 0, 0, err
		}
		typeIDs[typ.Name] = id
		typeCount++
	}

	// Insert implementation edges
	for _, impl := range impls {
		typeID, ok1 := typeIDs[impl.Type.Name]
		ifaceID, ok2 := interfaceIDs[impl.Interface.Name]
		if !ok1 || !ok2 {
			continue
		}

		edge := &graph.Edge{
			FromID: typeID,
			ToID:   ifaceID,
			Kind:   graph.EdgeKindImplements,
		}
		if err := insertEdgeFn(edge); err != nil {
			return 0, 0, 0, err
		}
		implCount++
	}

	return interfaceCount, typeCount, implCount, nil
}
