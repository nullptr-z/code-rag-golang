package impact

import (
	"fmt"
	"strings"

	"github.com/zheng/crag/internal/graph"
	"github.com/zheng/crag/internal/storage"
)

// Analyzer performs impact analysis on the code graph
type Analyzer struct {
	db *storage.DB
}

// NewAnalyzer creates a new impact analyzer
func NewAnalyzer(db *storage.DB) *Analyzer {
	return &Analyzer{db: db}
}

// ImpactReport represents the impact analysis of a function change
type ImpactReport struct {
	Target          *graph.Node   `json:"target"`
	DirectCallers   []*graph.Node `json:"direct_callers"`
	IndirectCallers []*graph.Node `json:"indirect_callers"`
	DirectCallees   []*graph.Node `json:"direct_callees"`
	IndirectCallees []*graph.Node `json:"indirect_callees"`
}

// AnalyzeImpact analyzes the impact of changing a function
func (a *Analyzer) AnalyzeImpact(funcName string, upstreamDepth, downstreamDepth int) (*ImpactReport, error) {
	// Find the target function
	target, err := a.db.GetNodeByName(funcName)
	if err != nil {
		// Try pattern matching if exact match fails
		nodes, err := a.db.FindNodesByPattern(funcName)
		if err != nil {
			return nil, fmt.Errorf("failed to find function: %w", err)
		}
		if len(nodes) == 0 {
			return nil, fmt.Errorf("function not found: %s", funcName)
		}
		if len(nodes) > 1 {
			var names []string
			for _, n := range nodes {
				names = append(names, n.Name)
			}
			return nil, fmt.Errorf("ambiguous function name, found %d matches: %s", len(nodes), strings.Join(names, ", "))
		}
		target = nodes[0]
	}

	report := &ImpactReport{
		Target: target,
	}

	// For var/const targets, find referencing functions instead of callers
	if target.Kind == graph.NodeKindVar || target.Kind == graph.NodeKindConst {
		report.DirectCallers, err = a.db.GetReferencingFunctions(target.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get referencing functions: %w", err)
		}
		// var/const don't call other functions
		return report, nil
	}

	// Get direct callers
	report.DirectCallers, err = a.db.GetDirectCallers(target.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get direct callers: %w", err)
	}

	// Get all upstream callers (indirect)
	if upstreamDepth != 1 {
		allCallers, err := a.db.GetUpstreamCallers(target.ID, upstreamDepth)
		if err != nil {
			return nil, fmt.Errorf("failed to get upstream callers: %w", err)
		}
		// Filter out direct callers to get indirect callers
		directMap := make(map[int64]bool)
		for _, c := range report.DirectCallers {
			directMap[c.ID] = true
		}
		for _, c := range allCallers {
			if !directMap[c.ID] {
				report.IndirectCallers = append(report.IndirectCallers, c)
			}
		}
	}

	// Get direct callees
	report.DirectCallees, err = a.db.GetDirectCallees(target.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get direct callees: %w", err)
	}

	// Get all downstream callees (indirect)
	if downstreamDepth != 1 {
		allCallees, err := a.db.GetDownstreamCallees(target.ID, downstreamDepth)
		if err != nil {
			return nil, fmt.Errorf("failed to get downstream callees: %w", err)
		}
		// Filter out direct callees to get indirect callees
		directMap := make(map[int64]bool)
		for _, c := range report.DirectCallees {
			directMap[c.ID] = true
		}
		for _, c := range allCallees {
			if !directMap[c.ID] {
				report.IndirectCallees = append(report.IndirectCallees, c)
			}
		}
	}

	return report, nil
}

// shortName simplifies a fully qualified function name
// e.g., "(*github.com/foo/bar/pkg.Type).Method" -> "(*pkg.Type).Method"
func shortName(fullName string) string {
	// Check for method receiver prefix like "(* or "("
	prefix := ""
	name := fullName
	if strings.HasPrefix(name, "(*") {
		prefix = "(*"
		name = name[2:]
	} else if strings.HasPrefix(name, "(") {
		prefix = "("
		name = name[1:]
	}

	// Find the last "/" and take everything after it
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	return prefix + name
}

// FormatMarkdown formats the impact report as markdown
func (r *ImpactReport) FormatMarkdown() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 变更影响分析: %s\n\n", shortName(r.Target.Name)))
	sb.WriteString(fmt.Sprintf("**位置:** %s:%d\n\n", r.Target.File, r.Target.Line))

	if r.Target.Signature != "" {
		sb.WriteString(fmt.Sprintf("**签名:** `%s`\n\n", r.Target.Signature))
	}

	if r.Target.Doc != "" {
		sb.WriteString(fmt.Sprintf("**文档:** %s\n\n", r.Target.Doc))
	}

	// Direct callers
	sb.WriteString("### 直接调用者 (需检查是否需要同步修改)\n\n")
	if len(r.DirectCallers) == 0 {
		sb.WriteString("_无直接调用者_\n\n")
	} else {
		sb.WriteString("| 函数 | 文件 | 行号 |\n")
		sb.WriteString("|------|------|------|\n")
		for _, c := range r.DirectCallers {
			sb.WriteString(fmt.Sprintf("| %s | %s | %d |\n", shortName(c.Name), c.File, c.Line))
		}
		sb.WriteString("\n")
	}

	// Indirect callers
	if len(r.IndirectCallers) > 0 {
		sb.WriteString("### 间接调用者 (可能受影响)\n\n")
		sb.WriteString("| 函数 | 文件 | 行号 |\n")
		sb.WriteString("|------|------|------|\n")
		for _, c := range r.IndirectCallers {
			sb.WriteString(fmt.Sprintf("| %s | %s | %d |\n", shortName(c.Name), c.File, c.Line))
		}
		sb.WriteString("\n")
	}

	// Direct callees
	sb.WriteString("### 下游依赖 (本函数调用的)\n\n")
	if len(r.DirectCallees) == 0 {
		sb.WriteString("_无下游依赖_\n\n")
	} else {
		sb.WriteString("| 函数 | 文件 | 行号 |\n")
		sb.WriteString("|------|------|------|\n")
		for _, c := range r.DirectCallees {
			sb.WriteString(fmt.Sprintf("| %s | %s | %d |\n", shortName(c.Name), c.File, c.Line))
		}
		sb.WriteString("\n")
	}

	// Indirect callees
	if len(r.IndirectCallees) > 0 {
		sb.WriteString("### 间接下游依赖\n\n")
		sb.WriteString("| 函数 | 文件 | 行号 |\n")
		sb.WriteString("|------|------|------|\n")
		for _, c := range r.IndirectCallees {
			sb.WriteString(fmt.Sprintf("| %s | %s | %d |\n", shortName(c.Name), c.File, c.Line))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatTree formats the impact report as a tree structure
func (r *ImpactReport) FormatTree() string {
	var sb strings.Builder

	// Collect all items and calculate max width for alignment
	allCallers := append(r.DirectCallers, r.IndirectCallers...)
	allCallees := append(r.DirectCallees, r.IndirectCallees...)

	maxWidth := len(fmt.Sprintf("%s:%d", shortPath(r.Target.File), r.Target.Line))
	for _, c := range allCallers {
		w := len(fmt.Sprintf("%s:%d", shortPath(c.File), c.Line))
		if w > maxWidth {
			maxWidth = w
		}
	}
	for _, c := range allCallees {
		w := len(fmt.Sprintf("%s:%d", shortPath(c.File), c.Line))
		if w > maxWidth {
			maxWidth = w
		}
	}

	// Target function
	sb.WriteString("📍 当前函数\n")
	sb.WriteString(fmt.Sprintf("%-*s  %s\n", maxWidth, fmt.Sprintf("%s:%d", shortPath(r.Target.File), r.Target.Line), shortName(r.Target.Name)))
	if r.Target.Signature != "" {
		sb.WriteString(fmt.Sprintf("   %s\n", r.Target.Signature))
	}
	sb.WriteString("\n")

	// Upstream callers
	callerCount := len(allCallers)
	if callerCount > 0 {
		sb.WriteString(fmt.Sprintf("⬆️ 调用者 (共 %d 个)\n", callerCount))
		for i, c := range allCallers {
			prefix := "├──"
			if i == len(allCallers)-1 {
				prefix = "└──"
			}
			loc := fmt.Sprintf("%s:%d", shortPath(c.File), c.Line)
			sb.WriteString(fmt.Sprintf("%s %-*s  %s\n", prefix, maxWidth, loc, shortName(c.Name)))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("⬆️ 调用者\n")
		sb.WriteString("└── (无)\n\n")
	}

	// Downstream callees
	calleeCount := len(allCallees)
	if calleeCount > 0 {
		sb.WriteString(fmt.Sprintf("⬇️ 被调用 (共 %d 个)\n", calleeCount))
		for i, c := range allCallees {
			prefix := "├──"
			if i == len(allCallees)-1 {
				prefix = "└──"
			}
			loc := fmt.Sprintf("%s:%d", shortPath(c.File), c.Line)
			sb.WriteString(fmt.Sprintf("%s %-*s  %s\n", prefix, maxWidth, loc, shortName(c.Name)))
		}
	} else {
		sb.WriteString("⬇️ 被调用\n")
		sb.WriteString("└── (无)\n")
	}

	return sb.String()
}

// shortPath extracts the last two path components
// e.g., "internal/livepk/livepk.go" -> "livepk/livepk.go"
func shortPath(fullPath string) string {
	parts := strings.Split(fullPath, "/")
	if len(parts) <= 2 {
		return fullPath
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

// Summary returns a brief summary of the impact report
func (r *ImpactReport) Summary() string {
	return fmt.Sprintf(
		"Target: %s, Direct Callers: %d, Indirect Callers: %d, Direct Callees: %d, Indirect Callees: %d",
		shortName(r.Target.Name),
		len(r.DirectCallers),
		len(r.IndirectCallers),
		len(r.DirectCallees),
		len(r.IndirectCallees),
	)
}

