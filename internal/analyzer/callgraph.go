package analyzer

import (
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/ssa"
)

// BuildCallGraph builds a static call graph for the program.
//
// The graph is used only for call edges. Function declarations are enumerated
// from package syntax by graph.Builder, so a call graph that omits an
// unreachable function cannot make that function disappear from the index.
func BuildCallGraph(prog *ssa.Program) (*callgraph.Graph, error) {
	return static.CallGraph(prog), nil
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
