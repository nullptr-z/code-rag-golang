package analyzer

import (
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// BuildCallGraph builds the call graph using VTA (Variable Type Analysis)
// VTA is more precise than other algorithms for handling interface calls
func BuildCallGraph(prog *ssa.Program) (*callgraph.Graph, error) {
	// Get all functions in the program
	funcs := ssautil.AllFunctions(prog)

	// Build call graph using VTA
	cg := vta.CallGraph(funcs, nil)

	return cg, nil
}

// CallGraphStats returns statistics about the call graph
type CallGraphStats struct {
	TotalNodes int
	TotalEdges int
}

// GetCallGraphStats returns statistics about the call graph
func GetCallGraphStats(cg *callgraph.Graph) CallGraphStats {
	stats := CallGraphStats{}

	// Count nodes
	for fn := range cg.Nodes {
		if fn != nil {
			stats.TotalNodes++
		}
	}

	// Count edges
	for _, node := range cg.Nodes {
		if node != nil {
			stats.TotalEdges += len(node.Out)
		}
	}

	return stats
}

