package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/zheng/crag/internal/impact"
	"github.com/zheng/crag/internal/storage"
)

// Server implements the MCP protocol for crag
type Server struct {
	db     *storage.DB
	input  io.Reader
	output io.Writer
}

// NewServer creates a new MCP server
func NewServer(db *storage.DB) *Server {
	return &Server{
		db:     db,
		input:  os.Stdin,
		output: os.Stdout,
	}
}

// JSON-RPC types
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP specific types
type InitializeParams struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ClientInfo      ClientInfo `json:"clientInfo"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Capabilities    Capabilities `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Default     interface{} `json:"default,omitempty"`
}

type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolCallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Run starts the MCP server
func (s *Server) Run() error {
	scanner := bufio.NewScanner(s.input)
	// Increase buffer size for large messages
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		s.handleRequest(&req)
	}

	return scanner.Err()
}

func (s *Server) handleRequest(req *Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		// Notification, no response needed
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	default:
		s.sendError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func (s *Server) handleInitialize(req *Request) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: ServerInfo{
			Name:    "crag",
			Version: "1.0.0",
		},
		Capabilities: Capabilities{
			Tools: &ToolsCapability{},
		},
	}
	s.sendResult(req.ID, result)
}

func (s *Server) handleToolsList(req *Request) {
	tools := []Tool{
		{
			Name: "impact",
			Description: `【重要】分析函数修改的影响范围。当用户要修改某个函数时，必须先调用此工具！
返回：
- 直接调用者：调用该函数的地方，修改参数/返回值时必须同步修改
- 间接调用者：可能受影响的上游函数
- 下游依赖：该函数调用的其他函数
使用场景：修改函数签名、重构函数、删除函数前`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"function": {
						Type:        "string",
						Description: "函数名，支持短名称如 'HandleRequest' 或完整名 'pkg/service.HandleRequest'",
					},
					"limit": {
						Type:        "number",
						Description: "每个分类最多返回数量，默认 50",
						Default:     50,
					},
				},
				Required: []string{"function"},
			},
		},
		{
			Name: "upstream",
			Description: `查询谁调用了这个函数（调用链向上追溯）。
使用场景：
- "这个函数在哪里被调用？"
- "修改这个函数会影响哪些地方？"
- 理解函数的使用方式和入口点`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"function": {
						Type:        "string",
						Description: "函数名，支持短名称如 'Query' 或 'db.Query'",
					},
					"depth": {
						Type:        "number",
						Description: "递归深度，0=无限，建议用2-3层",
					},
					"limit": {
						Type:        "number",
						Description: "最多返回数量，默认 50",
						Default:     50,
					},
				},
				Required: []string{"function"},
			},
		},
		{
			Name: "downstream",
			Description: `查询这个函数调用了什么（调用链向下追溯）。
使用场景：
- "这个函数内部调用了什么？"
- "这个函数的依赖是什么？"
- 理解函数的实现细节和依赖关系`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"function": {
						Type:        "string",
						Description: "函数名，支持短名称",
					},
					"depth": {
						Type:        "number",
						Description: "递归深度，0=无限，建议用2-3层",
					},
					"limit": {
						Type:        "number",
						Description: "最多返回数量，默认 50",
						Default:     50,
					},
				},
				Required: []string{"function"},
			},
		},
		{
			Name: "search",
			Description: `搜索项目中的函数。支持模糊匹配，短名称优先。
使用场景：
- 不确定函数完整名称时
- 查找包含某关键字的所有函数
- 探索项目结构
示例：搜索 'Handler' 会找到所有包含 Handler 的函数`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"pattern": {
						Type:        "string",
						Description: "搜索关键字，如 'Handler'、'Query'、'Process'",
					},
					"limit": {
						Type:        "number",
						Description: "最多返回数量，默认 50",
						Default:     50,
					},
				},
				Required: []string{"pattern"},
			},
		},
		{
			Name: "list",
			Description: `列出项目中的所有函数。用于了解项目整体结构。
使用场景：
- 初次了解项目时
- 查看项目有哪些主要函数
- 配合 offset 分页浏览`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"limit": {
						Type:        "number",
						Description: "返回数量，默认 50",
						Default:     50,
					},
					"offset": {
						Type:        "number",
						Description: "跳过前N个，用于分页",
						Default:     0,
					},
				},
			},
		},
		{
			Name: "mermaid",
			Description: `生成函数调用关系的 Mermaid 流程图。
使用场景：
- 用户想要可视化理解调用关系
- 生成文档或报告时
- 解释复杂的调用链`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"function": {
						Type:        "string",
						Description: "中心函数名",
					},
					"direction": {
						Type:        "string",
						Description: "upstream=上游调用者, downstream=下游被调用, both=双向（默认）",
					},
					"depth": {
						Type:        "number",
						Description: "展开深度，默认2",
					},
				},
				Required: []string{"function"},
			},
		},
	}

	s.sendResult(req.ID, map[string]interface{}{"tools": tools})
}

func (s *Server) handleToolsCall(req *Request) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32602, "Invalid params")
		return
	}

	var result string
	var isError bool

	switch params.Name {
	case "impact":
		result, isError = s.toolImpact(params.Arguments)
	case "upstream":
		result, isError = s.toolUpstream(params.Arguments)
	case "downstream":
		result, isError = s.toolDownstream(params.Arguments)
	case "search":
		result, isError = s.toolSearch(params.Arguments)
	case "list":
		result, isError = s.toolList(params.Arguments)
	case "mermaid":
		result, isError = s.toolMermaid(params.Arguments)
	default:
		result = fmt.Sprintf("Unknown tool: %s", params.Name)
		isError = true
	}

	s.sendResult(req.ID, ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: result}},
		IsError: isError,
	})
}

func (s *Server) toolImpact(args map[string]interface{}) (string, bool) {
	funcName, ok := args["function"].(string)
	if !ok || funcName == "" {
		return "错误：需要提供函数名称", true
	}

	limit := 50
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	analyzer := impact.NewAnalyzer(s.db)
	report, err := analyzer.AnalyzeImpact(funcName, 3, 2)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	return formatImpactWithLimit(report, limit), false
}

func formatImpactWithLimit(report *impact.ImpactReport, limit int) string {
	var result string

	result += fmt.Sprintf("## 变更影响分析: %s\n\n", report.Target.Name)
	result += fmt.Sprintf("**位置:** %s:%d\n\n", report.Target.File, report.Target.Line)

	if report.Target.Signature != "" {
		result += fmt.Sprintf("**签名:** `%s`\n\n", report.Target.Signature)
	}

	if report.Target.Doc != "" {
		result += fmt.Sprintf("**文档:** %s\n\n", report.Target.Doc)
	}

	// Direct callers
	result += "### 直接调用者 (需检查是否需要同步修改)\n\n"
	if len(report.DirectCallers) == 0 {
		result += "_无直接调用者_\n\n"
	} else {
		total := len(report.DirectCallers)
		callers := report.DirectCallers
		if len(callers) > limit {
			callers = callers[:limit]
		}
		result += "| 函数 | 文件 | 行号 |\n"
		result += "|------|------|------|\n"
		for _, c := range callers {
			result += fmt.Sprintf("| %s | %s | %d |\n", c.Name, c.File, c.Line)
		}
		if total > limit {
			result += fmt.Sprintf("\n_（共 %d 个，仅显示前 %d 个）_\n", total, limit)
		}
		result += "\n"
	}

	// Indirect callers
	if len(report.IndirectCallers) > 0 {
		result += "### 间接调用者 (可能受影响)\n\n"
		total := len(report.IndirectCallers)
		callers := report.IndirectCallers
		if len(callers) > limit {
			callers = callers[:limit]
		}
		result += "| 函数 | 文件 | 行号 |\n"
		result += "|------|------|------|\n"
		for _, c := range callers {
			result += fmt.Sprintf("| %s | %s | %d |\n", c.Name, c.File, c.Line)
		}
		if total > limit {
			result += fmt.Sprintf("\n_（共 %d 个，仅显示前 %d 个）_\n", total, limit)
		}
		result += "\n"
	}

	// Direct callees
	result += "### 下游依赖 (本函数调用的)\n\n"
	if len(report.DirectCallees) == 0 {
		result += "_无下游依赖_\n\n"
	} else {
		total := len(report.DirectCallees)
		callees := report.DirectCallees
		if len(callees) > limit {
			callees = callees[:limit]
		}
		result += "| 函数 | 文件 | 行号 |\n"
		result += "|------|------|------|\n"
		for _, c := range callees {
			result += fmt.Sprintf("| %s | %s | %d |\n", c.Name, c.File, c.Line)
		}
		if total > limit {
			result += fmt.Sprintf("\n_（共 %d 个，仅显示前 %d 个）_\n", total, limit)
		}
		result += "\n"
	}

	// Indirect callees
	if len(report.IndirectCallees) > 0 {
		result += "### 间接下游依赖\n\n"
		total := len(report.IndirectCallees)
		callees := report.IndirectCallees
		if len(callees) > limit {
			callees = callees[:limit]
		}
		result += "| 函数 | 文件 | 行号 |\n"
		result += "|------|------|------|\n"
		for _, c := range callees {
			result += fmt.Sprintf("| %s | %s | %d |\n", c.Name, c.File, c.Line)
		}
		if total > limit {
			result += fmt.Sprintf("\n_（共 %d 个，仅显示前 %d 个）_\n", total, limit)
		}
		result += "\n"
	}

	return result
}

func (s *Server) toolUpstream(args map[string]interface{}) (string, bool) {
	funcName, ok := args["function"].(string)
	if !ok || funcName == "" {
		return "错误：需要提供函数名称", true
	}

	depth := 0
	if d, ok := args["depth"].(float64); ok {
		depth = int(d)
	}

	limit := 50
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	// Find the function
	nodes, err := s.db.FindNodesByPattern(funcName)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}
	if len(nodes) == 0 {
		return fmt.Sprintf("未找到函数：%s\n\n💡 提示：如果这是新添加的函数，请运行以下命令更新数据库：\n```bash\ncrag analyze -i -r\n```", funcName), true
	}

	node := nodes[0]
	callers, err := s.db.GetUpstreamCallers(node.ID, depth)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	if len(callers) == 0 {
		return fmt.Sprintf("函数 %s 没有上游调用者", funcName), false
	}

	total := len(callers)
	if len(callers) > limit {
		callers = callers[:limit]
	}

	result := fmt.Sprintf("## %s 的上游调用者\n\n", funcName)
	result += "| 函数 | 文件 | 行号 |\n"
	result += "|------|------|------|\n"
	for _, c := range callers {
		result += fmt.Sprintf("| %s | %s | %d |\n", c.Name, c.File, c.Line)
	}

	if total > limit {
		result += fmt.Sprintf("\n_（共 %d 个，仅显示前 %d 个）_\n", total, limit)
	}

	return result, false
}

func (s *Server) toolDownstream(args map[string]interface{}) (string, bool) {
	funcName, ok := args["function"].(string)
	if !ok || funcName == "" {
		return "错误：需要提供函数名称", true
	}

	depth := 0
	if d, ok := args["depth"].(float64); ok {
		depth = int(d)
	}

	limit := 50
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	// Find the function
	nodes, err := s.db.FindNodesByPattern(funcName)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}
	if len(nodes) == 0 {
		return fmt.Sprintf("未找到函数：%s\n\n💡 提示：如果这是新添加的函数，请运行以下命令更新数据库：\n```bash\ncrag analyze -i -r\n```", funcName), true
	}

	node := nodes[0]
	callees, err := s.db.GetDownstreamCallees(node.ID, depth)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	if len(callees) == 0 {
		return fmt.Sprintf("函数 %s 没有下游调用", funcName), false
	}

	total := len(callees)
	if len(callees) > limit {
		callees = callees[:limit]
	}

	result := fmt.Sprintf("## %s 的下游调用\n\n", funcName)
	result += "| 函数 | 文件 | 行号 |\n"
	result += "|------|------|------|\n"
	for _, c := range callees {
		result += fmt.Sprintf("| %s | %s | %d |\n", c.Name, c.File, c.Line)
	}

	if total > limit {
		result += fmt.Sprintf("\n_（共 %d 个，仅显示前 %d 个）_\n", total, limit)
	}

	return result, false
}

func (s *Server) toolSearch(args map[string]interface{}) (string, bool) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return "错误：需要提供搜索模式", true
	}

	limit := 50
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	nodes, err := s.db.FindNodesByPattern(pattern)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	if len(nodes) == 0 {
		return fmt.Sprintf("未找到匹配 '%s' 的函数\n\n💡 提示：如果代码最近有更新，请运行以下命令更新数据库：\n```bash\ncrag analyze -i -r\n```", pattern), false
	}

	total := len(nodes)
	if len(nodes) > limit {
		nodes = nodes[:limit]
	}

	result := fmt.Sprintf("## 搜索结果：%s\n\n找到 %d 个匹配", pattern, total)
	if total > limit {
		result += fmt.Sprintf("（显示前 %d 个）", limit)
	}
	result += "\n\n"

	result += "| 函数 | 文件 | 行号 |\n"
	result += "|------|------|------|\n"
	for _, n := range nodes {
		result += fmt.Sprintf("| %s | %s | %d |\n", n.Name, n.File, n.Line)
	}

	return result, false
}

func (s *Server) toolList(args map[string]interface{}) (string, bool) {
	limit := 50
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	offset := 0
	if o, ok := args["offset"].(float64); ok && o > 0 {
		offset = int(o)
	}

	funcs, err := s.db.GetAllFunctions()
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	if len(funcs) == 0 {
		return "项目中没有函数", false
	}

	total := len(funcs)

	// Apply offset
	if offset >= total {
		return fmt.Sprintf("偏移量 %d 超出范围（共 %d 个函数）", offset, total), false
	}
	if offset > 0 {
		funcs = funcs[offset:]
	}

	// Apply limit
	displayed := len(funcs)
	if limit > 0 && limit < len(funcs) {
		funcs = funcs[:limit]
		displayed = limit
	}

	result := fmt.Sprintf("## 函数列表\n\n共 %d 个函数", total)
	if offset > 0 || displayed < total-offset {
		result += fmt.Sprintf("（显示 %d-%d）", offset+1, offset+displayed)
	}
	result += "\n\n"

	result += "| 函数 | 文件 | 行号 |\n"
	result += "|------|------|------|\n"
	for _, f := range funcs {
		result += fmt.Sprintf("| %s | %s | %d |\n", f.Name, f.File, f.Line)
	}

	return result, false
}

func (s *Server) toolMermaid(args map[string]interface{}) (string, bool) {
	funcName, ok := args["function"].(string)
	if !ok || funcName == "" {
		return "错误：需要提供函数名称", true
	}

	direction := "both"
	if d, ok := args["direction"].(string); ok && d != "" {
		direction = d
	}

	depth := 2
	if d, ok := args["depth"].(float64); ok && d > 0 {
		depth = int(d)
	}

	// Find the function
	nodes, err := s.db.FindNodesByPattern(funcName)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}
	if len(nodes) == 0 {
		return fmt.Sprintf("未找到函数：%s\n\n💡 提示：如果这是新添加的函数，请运行以下命令更新数据库：\n```bash\ncrag analyze -i -r\n```", funcName), true
	}

	node := nodes[0]

	// Build Mermaid diagram
	result := fmt.Sprintf("## %s 调用图\n\n", shortName(node.Name))
	result += "```mermaid\nflowchart TB\n"

	// Keep track of added nodes and edges to avoid duplicates
	addedNodes := make(map[int64]bool)
	addedEdges := make(map[string]bool)

	// Style the central node
	centerID := nodeID(node.Name)
	result += fmt.Sprintf("    %s[\"🎯 %s\"]\n", centerID, shortName(node.Name))
	result += fmt.Sprintf("    style %s fill:#f96,stroke:#333,stroke-width:2px\n", centerID)
	addedNodes[node.ID] = true

	// Get upstream callers
	if direction == "upstream" || direction == "both" {
		callers, _ := s.db.GetUpstreamCallers(node.ID, depth)
		for _, caller := range callers {
			if !addedNodes[caller.ID] {
				cID := nodeID(caller.Name)
				result += fmt.Sprintf("    %s[\"%s\"]\n", cID, shortName(caller.Name))
				result += fmt.Sprintf("    style %s fill:#9cf,stroke:#333\n", cID)
				addedNodes[caller.ID] = true
			}
		}
		// Add edges from callers to center
		directCallers, _ := s.db.GetDirectCallers(node.ID)
		for _, caller := range directCallers {
			edgeKey := fmt.Sprintf("%d->%d", caller.ID, node.ID)
			if !addedEdges[edgeKey] {
				result += fmt.Sprintf("    %s --> %s\n", nodeID(caller.Name), centerID)
				addedEdges[edgeKey] = true
			}
		}
		// Add edges between upstream nodes
		for _, caller := range callers {
			subCallers, _ := s.db.GetDirectCallers(caller.ID)
			for _, sc := range subCallers {
				if addedNodes[sc.ID] {
					edgeKey := fmt.Sprintf("%d->%d", sc.ID, caller.ID)
					if !addedEdges[edgeKey] {
						result += fmt.Sprintf("    %s --> %s\n", nodeID(sc.Name), nodeID(caller.Name))
						addedEdges[edgeKey] = true
					}
				}
			}
		}
	}

	// Get downstream callees
	if direction == "downstream" || direction == "both" {
		callees, _ := s.db.GetDownstreamCallees(node.ID, depth)
		for _, callee := range callees {
			if !addedNodes[callee.ID] {
				cID := nodeID(callee.Name)
				result += fmt.Sprintf("    %s[\"%s\"]\n", cID, shortName(callee.Name))
				result += fmt.Sprintf("    style %s fill:#9f9,stroke:#333\n", cID)
				addedNodes[callee.ID] = true
			}
		}
		// Add edges from center to callees
		directCallees, _ := s.db.GetDirectCallees(node.ID)
		for _, callee := range directCallees {
			edgeKey := fmt.Sprintf("%d->%d", node.ID, callee.ID)
			if !addedEdges[edgeKey] {
				result += fmt.Sprintf("    %s --> %s\n", centerID, nodeID(callee.Name))
				addedEdges[edgeKey] = true
			}
		}
		// Add edges between downstream nodes
		for _, callee := range callees {
			subCallees, _ := s.db.GetDirectCallees(callee.ID)
			for _, sc := range subCallees {
				if addedNodes[sc.ID] {
					edgeKey := fmt.Sprintf("%d->%d", callee.ID, sc.ID)
					if !addedEdges[edgeKey] {
						result += fmt.Sprintf("    %s --> %s\n", nodeID(callee.Name), nodeID(sc.Name))
						addedEdges[edgeKey] = true
					}
				}
			}
		}
	}

	result += "```\n\n"

	// Add legend
	result += "**图例说明:**\n"
	result += "- 🎯 橙色: 目标函数\n"
	if direction == "upstream" || direction == "both" {
		result += "- 蓝色: 上游调用者（调用目标函数）\n"
	}
	if direction == "downstream" || direction == "both" {
		result += "- 绿色: 下游被调用者（被目标函数调用）\n"
	}

	return result, false
}

// Helper functions for Mermaid generation
func shortName(fullName string) string {
	// Remove package prefix, keep receiver and method name
	name := fullName

	// Find the last package separator
	if idx := lastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	// Handle method receivers
	if len(name) > 2 && name[0] == '(' && name[1] == '*' {
		// (*Type).Method format
		if idx := indexOf(name, ")."); idx >= 0 {
			typePart := name[2:idx]
			if dotIdx := lastIndex(typePart, "."); dotIdx >= 0 {
				typePart = typePart[dotIdx+1:]
			}
			methodPart := name[idx+2:]
			return fmt.Sprintf("(*%s).%s", typePart, methodPart)
		}
	} else if len(name) > 1 && name[0] == '(' {
		// (Type).Method format
		if idx := indexOf(name, ")."); idx >= 0 {
			typePart := name[1:idx]
			if dotIdx := lastIndex(typePart, "."); dotIdx >= 0 {
				typePart = typePart[dotIdx+1:]
			}
			methodPart := name[idx+2:]
			return fmt.Sprintf("(%s).%s", typePart, methodPart)
		}
	}

	// Plain function - remove package prefix
	if dotIdx := lastIndex(name, "."); dotIdx >= 0 {
		return name[dotIdx+1:]
	}

	return name
}

func nodeID(name string) string {
	// Create a valid Mermaid node ID
	id := shortName(name)
	result := ""
	for _, c := range id {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			result += string(c)
		} else {
			result += "_"
		}
	}
	return result
}

func lastIndex(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func (s *Server) sendResult(id interface{}, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.send(resp)
}

func (s *Server) sendError(id interface{}, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &Error{Code: code, Message: message},
	}
	s.send(resp)
}

func (s *Server) send(resp Response) {
	data, _ := json.Marshal(resp)
	fmt.Fprintln(s.output, string(data))
}
