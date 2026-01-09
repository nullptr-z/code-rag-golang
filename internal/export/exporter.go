package export

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/zheng/crag/internal/graph"
	"github.com/zheng/crag/internal/storage"
)

// Exporter generates RAG documentation from the call graph database
type Exporter struct {
	db *storage.DB
}

// NewExporter creates a new exporter
func NewExporter(db *storage.DB) *Exporter {
	return &Exporter{db: db}
}

// ExportOptions configures the export behavior
type ExportOptions struct {
	IncludeMermaid    bool
	IncludeCallChains bool
	MaxDepth          int
	ProjectName       string
}

// DefaultExportOptions returns default export options
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		IncludeMermaid:    true,
		IncludeCallChains: true,
		MaxDepth:          2,
		ProjectName:       "项目",
	}
}

// Export generates a complete RAG document
func (e *Exporter) Export(w io.Writer, opts ExportOptions) error {
	funcs, err := e.db.GetAllFunctions()
	if err != nil {
		return fmt.Errorf("failed to get functions: %w", err)
	}

	nodeCount, edgeCount, _ := e.db.GetStats()
	pkgFuncs := groupByPackage(funcs)

	// Header
	fmt.Fprintf(w, "# %s调用图谱 (RAG)\n\n", opts.ProjectName)
	fmt.Fprintf(w, "> 生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "> 函数节点: %d | 调用边: %d\n\n", nodeCount, edgeCount)

	// Project structure
	e.writeProjectStructure(w, pkgFuncs)

	// Architecture diagram
	if opts.IncludeMermaid && len(funcs) > 0 {
		e.writeArchitectureDiagram(w, pkgFuncs)
	}

	// Package details
	fmt.Fprintf(w, "---\n\n## 模块详解\n\n")

	pkgNames := getSortedPackageNames(pkgFuncs)
	for _, pkg := range pkgNames {
		functions := pkgFuncs[pkg]
		e.writePackageSection(w, pkg, functions, opts)
	}

	// Impact reference table
	e.writeImpactTable(w, funcs)

	return nil
}

// writeProjectStructure writes the project directory structure
func (e *Exporter) writeProjectStructure(w io.Writer, pkgFuncs map[string][]*graph.Node) {
	fmt.Fprintf(w, "## 项目结构\n\n```\n")

	// Build directory tree from package paths
	dirs := make(map[string]bool)
	for pkg := range pkgFuncs {
		// Extract relative path from package
		parts := strings.Split(pkg, "/")
		if len(parts) > 2 {
			// Skip module prefix, keep internal paths
			for i := 2; i < len(parts); i++ {
				dir := strings.Join(parts[2:i+1], "/")
				dirs[dir] = true
			}
		}
	}

	// Sort and print
	var sortedDirs []string
	for dir := range dirs {
		sortedDirs = append(sortedDirs, dir)
	}
	sort.Strings(sortedDirs)

	for _, dir := range sortedDirs {
		indent := strings.Count(dir, "/")
		prefix := strings.Repeat("│   ", indent)
		name := filepath.Base(dir)
		fmt.Fprintf(w, "%s├── %s/\n", prefix, name)
	}

	fmt.Fprintf(w, "```\n\n")
}

// writeArchitectureDiagram writes a layered Mermaid architecture diagram
func (e *Exporter) writeArchitectureDiagram(w io.Writer, pkgFuncs map[string][]*graph.Node) {
	fmt.Fprintf(w, "## 架构图\n\n```mermaid\nflowchart TB\n")

	// Group packages into layers
	layers := e.categorizePackages(pkgFuncs)

	// Write subgraphs for each layer
	for _, layer := range []string{"cmd", "analyzer", "graph", "storage", "impact", "export", "other"} {
		pkgs, ok := layers[layer]
		if !ok || len(pkgs) == 0 {
			continue
		}

		layerName := getLayerDisplayName(layer)
		fmt.Fprintf(w, "    subgraph %s [%s]\n", layer, layerName)

		for _, pkg := range pkgs {
			functions := pkgFuncs[pkg]
			for _, fn := range functions {
				if isKeyFunction(fn.Name) {
					nodeID := makeNodeID(fn.Name)
					displayName := getShortDisplayName(fn.Name)
					fmt.Fprintf(w, "        %s[%s]\n", nodeID, displayName)
				}
			}
		}

		fmt.Fprintf(w, "    end\n\n")
	}

	// Write key edges between layers
	fmt.Fprintf(w, "    %% 关键调用关系\n")
	for pkg := range pkgFuncs {
		functions := pkgFuncs[pkg]
		for _, fn := range functions {
			if !isKeyFunction(fn.Name) {
				continue
			}
			callees, _ := e.db.GetDirectCallees(fn.ID)
			fromID := makeNodeID(fn.Name)
			for _, callee := range callees {
				if isKeyFunction(callee.Name) {
					toID := makeNodeID(callee.Name)
					fmt.Fprintf(w, "    %s --> %s\n", fromID, toID)
				}
			}
		}
	}

	fmt.Fprintf(w, "```\n\n")
}

// writePackageSection writes detailed info for a package
func (e *Exporter) writePackageSection(w io.Writer, pkg string, functions []*graph.Node, opts ExportOptions) {
	shortPkg := getShortPackageName(pkg)
	fmt.Fprintf(w, "### 📦 %s\n\n", shortPkg)

	// Sort by function type (exported first) then name
	sort.Slice(functions, func(i, j int) bool {
		iExp := isExportedFunc(functions[i].Name)
		jExp := isExportedFunc(functions[j].Name)
		if iExp != jExp {
			return iExp
		}
		return functions[i].Name < functions[j].Name
	})

	// Table header
	fmt.Fprintf(w, "| 函数 | 说明 | 被调用 | 调用 |\n")
	fmt.Fprintf(w, "|------|------|--------|------|\n")

	for _, fn := range functions {
		shortName := getShortDisplayName(fn.Name)
		doc := truncateDoc(fn.Doc, 30)
		if doc == "" {
			doc = "-"
		}

		callers, _ := e.db.GetDirectCallers(fn.ID)
		callees, _ := e.db.GetDirectCallees(fn.ID)

		callerCount := len(callers)
		calleeCount := len(callees)

		fmt.Fprintf(w, "| `%s` | %s | %d | %d |\n", shortName, doc, callerCount, calleeCount)
	}

	fmt.Fprintf(w, "\n")

	// Detailed function info for key functions
	for _, fn := range functions {
		if !isKeyFunction(fn.Name) {
			continue
		}

		shortName := getShortDisplayName(fn.Name)
		fmt.Fprintf(w, "#### `%s`\n\n", shortName)
		fmt.Fprintf(w, "- **位置**: `%s:%d`\n", getRelativePath(fn.File), fn.Line)

		if fn.Signature != "" {
			fmt.Fprintf(w, "- **签名**: `%s`\n", fn.Signature)
		}

		if fn.Doc != "" {
			fmt.Fprintf(w, "- **说明**: %s\n", fn.Doc)
		}

		if opts.IncludeCallChains {
			callers, _ := e.db.GetDirectCallers(fn.ID)
			callees, _ := e.db.GetDirectCallees(fn.ID)

			if len(callers) > 0 {
				fmt.Fprintf(w, "- **被调用**: ")
				var names []string
				for _, c := range callers {
					names = append(names, "`"+getShortDisplayName(c.Name)+"`")
				}
				fmt.Fprintf(w, "%s\n", strings.Join(names, ", "))
			}

			if len(callees) > 0 {
				fmt.Fprintf(w, "- **调用**: ")
				var names []string
				for _, c := range callees {
					names = append(names, "`"+getShortDisplayName(c.Name)+"`")
				}
				fmt.Fprintf(w, "%s\n", strings.Join(names, ", "))
			}
		}

		fmt.Fprintf(w, "\n")
	}
}

// writeImpactTable writes a summary table for impact analysis
func (e *Exporter) writeImpactTable(w io.Writer, funcs []*graph.Node) {
	fmt.Fprintf(w, "---\n\n## 修改影响速查\n\n")
	fmt.Fprintf(w, "| 函数 | 位置 | 被调用次数 | 调用次数 | 风险 |\n")
	fmt.Fprintf(w, "|------|------|-----------|----------|------|\n")

	// Sort by caller count (most called first)
	type funcWithStats struct {
		fn      *graph.Node
		callers int
		callees int
	}

	var stats []funcWithStats
	for _, fn := range funcs {
		callers, _ := e.db.GetDirectCallers(fn.ID)
		callees, _ := e.db.GetDirectCallees(fn.ID)
		if len(callers) > 0 {
			stats = append(stats, funcWithStats{fn, len(callers), len(callees)})
		}
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].callers > stats[j].callers
	})

	for _, s := range stats {
		risk := "🟢"
		if s.callers >= 5 {
			risk = "🔴 高"
		} else if s.callers >= 3 {
			risk = "🟡 中"
		}

		fmt.Fprintf(w, "| `%s` | %s:%d | %d | %d | %s |\n",
			getShortDisplayName(s.fn.Name),
			getRelativePath(s.fn.File),
			s.fn.Line,
			s.callers,
			s.callees,
			risk,
		)
	}
}

// ExportIncremental generates a RAG document for changed packages only
func (e *Exporter) ExportIncremental(w io.Writer, changedPackages []string, opts ExportOptions) error {
	if len(changedPackages) == 0 {
		fmt.Fprintf(w, "# 增量更新报告\n\n> 没有检测到变更\n")
		return nil
	}

	funcs, err := e.db.GetAllFunctions()
	if err != nil {
		return fmt.Errorf("failed to get functions: %w", err)
	}

	// Filter to changed packages
	var changedFuncs []*graph.Node
	for _, fn := range funcs {
		for _, pkg := range changedPackages {
			if matchPackageSuffix(fn.Package, pkg) {
				changedFuncs = append(changedFuncs, fn)
				break
			}
		}
	}

	fmt.Fprintf(w, "# 增量更新报告\n\n")
	fmt.Fprintf(w, "> 生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "> 变更包: %d | 变更函数: %d\n\n", len(changedPackages), len(changedFuncs))

	fmt.Fprintf(w, "## 变更范围\n\n")
	for _, pkg := range changedPackages {
		fmt.Fprintf(w, "- `%s`\n", pkg)
	}
	fmt.Fprintf(w, "\n")

	if len(changedFuncs) == 0 {
		fmt.Fprintf(w, "_没有受影响的函数_\n")
		return nil
	}

	fmt.Fprintf(w, "## 影响分析\n\n")

	for _, fn := range changedFuncs {
		shortName := getShortDisplayName(fn.Name)
		callers, _ := e.db.GetDirectCallers(fn.ID)

		if len(callers) == 0 {
			continue
		}

		fmt.Fprintf(w, "### ⚠️ `%s`\n\n", shortName)
		fmt.Fprintf(w, "**位置**: `%s:%d`\n\n", getRelativePath(fn.File), fn.Line)
		fmt.Fprintf(w, "**以下 %d 个函数调用了此函数，可能需要检查：**\n\n", len(callers))
		fmt.Fprintf(w, "| 调用者 | 文件 | 行号 |\n")
		fmt.Fprintf(w, "|--------|------|------|\n")
		for _, c := range callers {
			fmt.Fprintf(w, "| `%s` | %s | %d |\n",
				getShortDisplayName(c.Name),
				getRelativePath(c.File),
				c.Line,
			)
		}
		fmt.Fprintf(w, "\n")
	}

	return nil
}

// Helper functions

func (e *Exporter) categorizePackages(pkgFuncs map[string][]*graph.Node) map[string][]string {
	layers := make(map[string][]string)

	for pkg := range pkgFuncs {
		layer := "other"
		if strings.Contains(pkg, "/cmd/") {
			layer = "cmd"
		} else if strings.Contains(pkg, "/analyzer") {
			layer = "analyzer"
		} else if strings.Contains(pkg, "/graph") {
			layer = "graph"
		} else if strings.Contains(pkg, "/storage") {
			layer = "storage"
		} else if strings.Contains(pkg, "/impact") {
			layer = "impact"
		} else if strings.Contains(pkg, "/export") {
			layer = "export"
		}
		layers[layer] = append(layers[layer], pkg)
	}

	return layers
}

func groupByPackage(funcs []*graph.Node) map[string][]*graph.Node {
	result := make(map[string][]*graph.Node)
	for _, fn := range funcs {
		result[fn.Package] = append(result[fn.Package], fn)
	}
	return result
}

func getSortedPackageNames(pkgFuncs map[string][]*graph.Node) []string {
	var names []string
	for pkg := range pkgFuncs {
		names = append(names, pkg)
	}
	// Sort by layer order
	sort.Slice(names, func(i, j int) bool {
		return getLayerOrder(names[i]) < getLayerOrder(names[j])
	})
	return names
}

func getLayerOrder(pkg string) int {
	if strings.Contains(pkg, "/cmd/") {
		return 0
	} else if strings.Contains(pkg, "/analyzer") {
		return 1
	} else if strings.Contains(pkg, "/graph") {
		return 2
	} else if strings.Contains(pkg, "/storage") {
		return 3
	} else if strings.Contains(pkg, "/impact") {
		return 4
	} else if strings.Contains(pkg, "/export") {
		return 5
	}
	return 9
}

func getLayerDisplayName(layer string) string {
	switch layer {
	case "cmd":
		return "CLI 命令层"
	case "analyzer":
		return "静态分析层"
	case "graph":
		return "图构建层"
	case "storage":
		return "存储层"
	case "impact":
		return "影响分析层"
	case "export":
		return "导出层"
	default:
		return "其他"
	}
}

func getShortPackageName(pkg string) string {
	parts := strings.Split(pkg, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return pkg
}

func getShortDisplayName(fullName string) string {
	// Remove package prefix, keep receiver and method name
	// e.g., "(*github.com/zheng/crag/internal/storage.DB).GetStats" -> "(*DB).GetStats"
	name := fullName

	// Find the last package separator
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	// Handle method receivers
	if strings.HasPrefix(name, "(*") {
		// Find the dot before method name
		if idx := strings.Index(name, ")."); idx >= 0 {
			// Extract type name (after package prefix if any)
			typePart := name[2:idx]
			if dotIdx := strings.LastIndex(typePart, "."); dotIdx >= 0 {
				typePart = typePart[dotIdx+1:]
			}
			methodPart := name[idx+2:]
			return fmt.Sprintf("(*%s).%s", typePart, methodPart)
		}
	} else if strings.HasPrefix(name, "(") {
		// Value receiver
		if idx := strings.Index(name, ")."); idx >= 0 {
			typePart := name[1:idx]
			if dotIdx := strings.LastIndex(typePart, "."); dotIdx >= 0 {
				typePart = typePart[dotIdx+1:]
			}
			methodPart := name[idx+2:]
			return fmt.Sprintf("(%s).%s", typePart, methodPart)
		}
	}

	// Plain function - remove package prefix
	if dotIdx := strings.LastIndex(name, "."); dotIdx >= 0 {
		return name[dotIdx+1:]
	}

	return name
}

func makeNodeID(name string) string {
	// Create a valid Mermaid node ID
	id := getShortDisplayName(name)
	id = strings.ReplaceAll(id, "(", "")
	id = strings.ReplaceAll(id, ")", "")
	id = strings.ReplaceAll(id, "*", "")
	id = strings.ReplaceAll(id, ".", "_")
	id = strings.ReplaceAll(id, "$", "_")
	return id
}

func getRelativePath(path string) string {
	// Try to get relative path from common patterns
	if idx := strings.Index(path, "/internal/"); idx >= 0 {
		return path[idx+1:]
	}
	if idx := strings.Index(path, "/cmd/"); idx >= 0 {
		return path[idx+1:]
	}
	return filepath.Base(path)
}

func isKeyFunction(name string) bool {
	// Skip anonymous functions and closures
	if strings.Contains(name, "$") {
		return false
	}
	// Skip helper functions (lowercase unexported)
	shortName := getShortDisplayName(name)
	if len(shortName) > 0 {
		first := shortName[0]
		if first >= 'a' && first <= 'z' {
			return false
		}
		// Method receivers start with (
		if first == '(' && len(shortName) > 2 {
			// Check if method name is exported
			if idx := strings.Index(shortName, ")."); idx >= 0 && idx+2 < len(shortName) {
				methodFirst := shortName[idx+2]
				return methodFirst >= 'A' && methodFirst <= 'Z'
			}
		}
	}
	return true
}

func isExportedFunc(name string) bool {
	shortName := getShortDisplayName(name)
	if len(shortName) == 0 {
		return false
	}
	first := shortName[0]
	if first >= 'A' && first <= 'Z' {
		return true
	}
	// Check method name for receivers
	if strings.HasPrefix(shortName, "(*") || strings.HasPrefix(shortName, "(") {
		if idx := strings.Index(shortName, ")."); idx >= 0 && idx+2 < len(shortName) {
			methodFirst := shortName[idx+2]
			return methodFirst >= 'A' && methodFirst <= 'Z'
		}
	}
	return false
}

func truncateDoc(doc string, maxLen int) string {
	doc = strings.TrimSpace(doc)
	// Take first line only
	if idx := strings.Index(doc, "\n"); idx >= 0 {
		doc = doc[:idx]
	}
	if len(doc) > maxLen {
		return doc[:maxLen-3] + "..."
	}
	return doc
}

func matchPackageSuffix(fullPath, pattern string) bool {
	if len(pattern) > 2 && pattern[:2] == "./" {
		pattern = pattern[2:]
	}
	return strings.HasSuffix(fullPath, pattern)
}

func containsPackage(packages []string, pkg string) bool {
	for _, p := range packages {
		if p == pkg || matchPackageSuffix(pkg, p) {
			return true
		}
	}
	return false
}
