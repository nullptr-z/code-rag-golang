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

// FormatMarkdown formats the impact report as markdown
func (r *ImpactReport) FormatMarkdown() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 变更影响分析: %s\n\n", r.Target.Name))
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
			sb.WriteString(fmt.Sprintf("| %s | %s | %d |\n", c.Name, c.File, c.Line))
		}
		sb.WriteString("\n")
	}

	// Indirect callers
	if len(r.IndirectCallers) > 0 {
		sb.WriteString("### 间接调用者 (可能受影响)\n\n")
		sb.WriteString("| 函数 | 文件 | 行号 |\n")
		sb.WriteString("|------|------|------|\n")
		for _, c := range r.IndirectCallers {
			sb.WriteString(fmt.Sprintf("| %s | %s | %d |\n", c.Name, c.File, c.Line))
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
			sb.WriteString(fmt.Sprintf("| %s | %s | %d |\n", c.Name, c.File, c.Line))
		}
		sb.WriteString("\n")
	}

	// Indirect callees
	if len(r.IndirectCallees) > 0 {
		sb.WriteString("### 间接下游依赖\n\n")
		sb.WriteString("| 函数 | 文件 | 行号 |\n")
		sb.WriteString("|------|------|------|\n")
		for _, c := range r.IndirectCallees {
			sb.WriteString(fmt.Sprintf("| %s | %s | %d |\n", c.Name, c.File, c.Line))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatJSON formats the impact report as JSON (use encoding/json for actual serialization)
func (r *ImpactReport) Summary() string {
	return fmt.Sprintf(
		"Target: %s, Direct Callers: %d, Indirect Callers: %d, Direct Callees: %d, Indirect Callees: %d",
		r.Target.Name,
		len(r.DirectCallers),
		len(r.IndirectCallers),
		len(r.DirectCallees),
		len(r.IndirectCallees),
	)
}

