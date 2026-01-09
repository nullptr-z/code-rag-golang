package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/zheng/crag/internal/graph"
	"github.com/zheng/crag/internal/storage"
)

//go:embed static/*
var staticFS embed.FS

// Server is the web server for visualizing call graphs
type Server struct {
	db   *storage.DB
	port int
}

// NewServer creates a new web server
func NewServer(db *storage.DB, port int) *Server {
	return &Server{db: db, port: port}
}

// API response types
type GraphData struct {
	Nodes []NodeData `json:"nodes"`
	Edges []EdgeData `json:"edges"`
}

type NodeData struct {
	ID        int64  `json:"id"`
	Label     string `json:"label"`
	FullName  string `json:"fullName"`
	Package   string `json:"package"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Signature string `json:"signature"`
	Doc       string `json:"doc"`
	Group     string `json:"group"`
}

type EdgeData struct {
	From         int64  `json:"from"`
	To           int64  `json:"to"`
	Kind         string `json:"kind"`
	CallSiteLine int    `json:"callSiteLine"` // 调用发生的行号，用于排序执行顺序
}

type ImpactData struct {
	Target     NodeData   `json:"target"`
	Upstream   []NodeData `json:"upstream"`
	Downstream []NodeData `json:"downstream"`
}

// CallChainNode represents a node in the hierarchical call chain
type CallChainNode struct {
	NodeData
	Children []CallChainNode `json:"children,omitempty"`
}

// CallChainData represents hierarchical call chain data
type CallChainData struct {
	Target  NodeData        `json:"target"`
	Callers []CallChainNode `json:"callers"` // Who calls this function (upstream)
	Callees []CallChainNode `json:"callees"` // What this function calls (downstream)
}

type StatsData struct {
	NodeCount int64 `json:"nodeCount"`
	EdgeCount int64 `json:"edgeCount"`
}

// Run starts the web server
func (s *Server) Run() error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/graph", s.handleGraph)
	mux.HandleFunc("/api/nodes", s.handleNodes)
	mux.HandleFunc("/api/node/", s.handleNode)
	mux.HandleFunc("/api/impact/", s.handleImpact)
	mux.HandleFunc("/api/chain/", s.handleCallChain)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/stats", s.handleStats)

	// Static files
	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("failed to get static files: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticContent)))

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("🌐 Web UI 启动: http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}

// handleGraph returns the complete graph data
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	funcs, err := s.db.GetAllFunctions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	edges, err := s.db.GetAllEdges()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := GraphData{
		Nodes: make([]NodeData, 0, len(funcs)),
		Edges: make([]EdgeData, 0, len(edges)),
	}

	for _, fn := range funcs {
		data.Nodes = append(data.Nodes, nodeToData(fn))
	}

	for _, edge := range edges {
		data.Edges = append(data.Edges, EdgeData{
			From:         edge.FromID,
			To:           edge.ToID,
			Kind:         string(edge.Kind),
			CallSiteLine: edge.CallSiteLine,
		})
	}

	writeJSON(w, data)
}

// handleNodes returns all nodes
func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	funcs, err := s.db.GetAllFunctions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	nodes := make([]NodeData, 0, len(funcs))
	for _, fn := range funcs {
		nodes = append(nodes, nodeToData(fn))
	}

	writeJSON(w, nodes)
}

// handleNode returns a single node with its connections
func (s *Server) handleNode(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/node/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	node, err := s.db.GetNodeByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if node == nil {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	callers, _ := s.db.GetDirectCallers(id)
	callees, _ := s.db.GetDirectCallees(id)

	result := map[string]interface{}{
		"node":    nodeToData(node),
		"callers": nodesToData(callers),
		"callees": nodesToData(callees),
	}

	writeJSON(w, result)
}

// handleImpact returns impact analysis for a node
func (s *Server) handleImpact(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/impact/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	depth := 3
	if d := r.URL.Query().Get("depth"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil {
			depth = parsed
		}
	}

	node, err := s.db.GetNodeByID(id)
	if err != nil || node == nil {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	upstream, _ := s.db.GetUpstreamCallers(id, depth)
	downstream, _ := s.db.GetDownstreamCallees(id, depth)

	data := ImpactData{
		Target:     nodeToData(node),
		Upstream:   nodesToData(upstream),
		Downstream: nodesToData(downstream),
	}

	writeJSON(w, data)
}

// handleCallChain returns hierarchical call chain for a node
func (s *Server) handleCallChain(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/chain/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	depth := 2
	if d := r.URL.Query().Get("depth"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil {
			depth = parsed
		}
	}

	node, err := s.db.GetNodeByID(id)
	if err != nil || node == nil {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Build hierarchical callers (upstream)
	callers := s.buildCallersTree(id, depth, make(map[int64]bool))

	// Build hierarchical callees (downstream)
	callees := s.buildCalleesTree(id, depth, make(map[int64]bool))

	data := CallChainData{
		Target:  nodeToData(node),
		Callers: callers,
		Callees: callees,
	}

	writeJSON(w, data)
}

// buildCallersTree recursively builds the callers tree
func (s *Server) buildCallersTree(nodeID int64, depth int, visited map[int64]bool) []CallChainNode {
	if depth <= 0 {
		return nil
	}

	callers, err := s.db.GetDirectCallers(nodeID)
	if err != nil || len(callers) == 0 {
		return nil
	}

	result := make([]CallChainNode, 0, len(callers))
	for _, caller := range callers {
		if visited[caller.ID] {
			continue // Avoid cycles
		}
		visited[caller.ID] = true

		chainNode := CallChainNode{
			NodeData: nodeToData(caller),
		}

		// Recursively get callers of this caller
		if depth > 1 {
			chainNode.Children = s.buildCallersTree(caller.ID, depth-1, visited)
		}

		result = append(result, chainNode)
	}

	return result
}

// buildCalleesTree recursively builds the callees tree
func (s *Server) buildCalleesTree(nodeID int64, depth int, visited map[int64]bool) []CallChainNode {
	if depth <= 0 {
		return nil
	}

	callees, err := s.db.GetDirectCallees(nodeID)
	if err != nil || len(callees) == 0 {
		return nil
	}

	result := make([]CallChainNode, 0, len(callees))
	for _, callee := range callees {
		if visited[callee.ID] {
			continue // Avoid cycles
		}
		visited[callee.ID] = true

		chainNode := CallChainNode{
			NodeData: nodeToData(callee),
		}

		// Recursively get callees of this callee
		if depth > 1 {
			chainNode.Children = s.buildCalleesTree(callee.ID, depth-1, visited)
		}

		result = append(result, chainNode)
	}

	return result
}

// handleSearch searches for nodes by pattern
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	pattern := r.URL.Query().Get("q")
	if pattern == "" {
		writeJSON(w, []NodeData{})
		return
	}

	nodes, err := s.db.FindNodesByPattern(pattern)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, nodesToData(nodes))
}

// handleStats returns database statistics
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	nodeCount, edgeCount, _ := s.db.GetStats()
	writeJSON(w, StatsData{
		NodeCount: nodeCount,
		EdgeCount: edgeCount,
	})
}

// Helper functions
func nodeToData(n *graph.Node) NodeData {
	return NodeData{
		ID:        n.ID,
		Label:     shortName(n.Name),
		FullName:  n.Name,
		Package:   n.Package,
		File:      n.File,
		Line:      n.Line,
		Signature: n.Signature,
		Doc:       n.Doc,
		Group:     getPackageGroup(n.Package),
	}
}

func nodesToData(nodes []*graph.Node) []NodeData {
	result := make([]NodeData, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, nodeToData(n))
	}
	return result
}

func shortName(fullName string) string {
	name := fullName
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	// Handle method receivers
	if len(name) > 2 && name[0] == '(' && name[1] == '*' {
		if idx := strings.Index(name, ")."); idx >= 0 {
			typePart := name[2:idx]
			if dotIdx := strings.LastIndex(typePart, "."); dotIdx >= 0 {
				typePart = typePart[dotIdx+1:]
			}
			methodPart := name[idx+2:]
			return fmt.Sprintf("(*%s).%s", typePart, methodPart)
		}
	} else if len(name) > 1 && name[0] == '(' {
		if idx := strings.Index(name, ")."); idx >= 0 {
			typePart := name[1:idx]
			if dotIdx := strings.LastIndex(typePart, "."); dotIdx >= 0 {
				typePart = typePart[dotIdx+1:]
			}
			methodPart := name[idx+2:]
			return fmt.Sprintf("(%s).%s", typePart, methodPart)
		}
	}

	if dotIdx := strings.LastIndex(name, "."); dotIdx >= 0 {
		return name[dotIdx+1:]
	}
	return name
}

func getPackageGroup(pkg string) string {
	if strings.Contains(pkg, "/cmd/") {
		return "cmd"
	} else if strings.Contains(pkg, "/analyzer") {
		return "analyzer"
	} else if strings.Contains(pkg, "/graph") {
		return "graph"
	} else if strings.Contains(pkg, "/storage") {
		return "storage"
	} else if strings.Contains(pkg, "/impact") {
		return "impact"
	} else if strings.Contains(pkg, "/export") {
		return "export"
	} else if strings.Contains(pkg, "/web") {
		return "web"
	} else if strings.Contains(pkg, "/mcp") {
		return "mcp"
	}
	return "other"
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(data)
}
