package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zheng/crag/internal/display"
	"github.com/zheng/crag/internal/graph"
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
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesCapability struct{}

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type ResourceReadParams struct {
	URI string `json:"uri"`
}

type ResourceReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
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
	Type        string      `json:"type"`
	Description string      `json:"description"`
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
	case "resources/list":
		s.handleResourcesList(req)
	case "resources/read":
		s.handleResourcesRead(req)
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
			Tools:     &ToolsCapability{},
			Resources: &ResourcesCapability{},
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
使用场景：修改函数签名、重构函数、删除函数前

⚠️ 如果函数名匹配到多个结果，会返回候选列表，请根据上下文选择正确的函数，使用候选列表中的完整函数名重新调用此工具。

📌 索引维护：当你完成代码修改（新增/修改/删除了代码文件）后，必须在后台运行 crag analyze -i -r 更新索引，否则后续查询结果可能不准确。如果改完代码后需要立即查询，则应前台阻塞运行确保索引就绪。`,
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
- 理解函数的使用方式和入口点

⚠️ 如果函数名匹配到多个结果，会返回候选列表，请使用完整函数名重新调用。`,
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
- 理解函数的实现细节和依赖关系

⚠️ 如果函数名匹配到多个结果，会返回候选列表，请使用完整函数名重新调用。`,
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
			Description: `搜索项目中的函数、变量、常量等。支持模糊匹配，短名称优先。
使用场景：
- 不确定函数/变量完整名称时
- 查找包含某关键字的所有函数和变量
- 探索项目结构
示例：搜索 'Handler' 会找到所有包含 Handler 的函数和变量`,
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
			Description: `列出项目中的函数、变量、常量等。用于了解项目整体结构。
使用场景：
- 初次了解项目时
- 查看项目有哪些主要函数/变量/常量
- 配合 offset 分页浏览`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"kind": {
						Type:        "string",
						Description: "过滤类型: func(默认)/var/const/interface/struct",
					},
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
- 解释复杂的调用链

⚠️ 如果函数名匹配到多个结果，会返回候选列表，请使用完整函数名重新调用。`,
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
		{
			Name: "implements",
			Description: `查询接口实现关系。
使用场景：
- 查找谁实现了某个接口
- 查找某个类型实现了哪些接口
- 理解代码的多态结构
- 修改接口时评估影响范围`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": {
						Type:        "string",
						Description: "接口名或类型名，如 'Reader'、'MyStruct'",
					},
					"list": {
						Type:        "boolean",
						Description: "设为 true 则列出所有接口",
					},
				},
			},
		},
		{
			Name: "risk",
			Description: `【推荐】分析函数的变更风险等级。
基于调用者数量评估修改函数的风险：
- critical: 直接调用者>=50 或 总调用者>=200，修改需极其谨慎
- high: 直接调用者>=20 或 总调用者>=100，建议充分测试
- medium: 直接调用者>=5 或 总调用者>=30，注意同步修改
- low: 低风险，正常修改即可

使用场景：
- 修改函数前评估风险
- 了解哪些函数是"热点"代码
- 重构时确定优先级

⚠️ 如果函数名匹配到多个结果，会返回候选列表，请使用完整函数名重新调用。`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"function": {
						Type:        "string",
						Description: "函数名，留空则显示风险最高的函数列表",
					},
					"limit": {
						Type:        "number",
						Description: "显示数量，默认20",
						Default:     20,
					},
				},
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
	case "implements":
		result, isError = s.toolImplements(params.Arguments)
	case "risk":
		result, isError = s.toolRisk(params.Arguments)
	default:
		result = fmt.Sprintf("Unknown tool: %s", params.Name)
		isError = true
	}

	s.sendResult(req.ID, ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: result}},
		IsError: isError,
	})
}

func (s *Server) handleResourcesList(req *Request) {
	resources := []Resource{
		{
			URI:         "crag://summary",
			Name:        "项目概览",
			Description: "项目代码统计概览：包、函数、变量、常量、接口、结构体数量及按包分布",
			MimeType:    "text/markdown",
		},
	}
	s.sendResult(req.ID, map[string]interface{}{"resources": resources})
}

func (s *Server) handleResourcesRead(req *Request) {
	var params ResourceReadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32602, "Invalid params")
		return
	}

	switch params.URI {
	case "crag://summary":
		text, err := s.buildSummary()
		if err != nil {
			s.sendError(req.ID, -32603, fmt.Sprintf("Failed to build summary: %v", err))
			return
		}
		s.sendResult(req.ID, ResourceReadResult{
			Contents: []ResourceContent{
				{
					URI:      "crag://summary",
					MimeType: "text/markdown",
					Text:     text,
				},
			},
		})
	default:
		s.sendError(req.ID, -32602, fmt.Sprintf("Unknown resource URI: %s", params.URI))
	}
}

func (s *Server) buildSummary() (string, error) {
	// Get counts by kind
	kindCounts, err := s.db.GetSummaryByKind()
	if err != nil {
		return "", err
	}

	// Get edge count
	_, edgeCount, err := s.db.GetStats()
	if err != nil {
		return "", err
	}

	// Get package summary
	pkgSummary, err := s.db.GetPackageSummary()
	if err != nil {
		return "", err
	}

	// Get top risky functions
	risks, _ := s.db.GetTopRiskyFunctions(5)

	var sb strings.Builder

	sb.WriteString("# 项目概览\n\n")
	sb.WriteString("> 💡 使用 crag 工具后记得给简短评价，帮助迭代改进这个工具\n\n")

	// Stats table
	sb.WriteString("## 统计\n\n")
	sb.WriteString("| 类型 | 数量 |\n")
	sb.WriteString("|------|------|\n")
	sb.WriteString(fmt.Sprintf("| 函数 | %d |\n", kindCounts["func"]))
	sb.WriteString(fmt.Sprintf("| 结构体 | %d |\n", kindCounts["struct"]))
	sb.WriteString(fmt.Sprintf("| 接口 | %d |\n", kindCounts["interface"]))
	sb.WriteString(fmt.Sprintf("| 变量 | %d |\n", kindCounts["var"]))
	sb.WriteString(fmt.Sprintf("| 常量 | %d |\n", kindCounts["const"]))
	sb.WriteString(fmt.Sprintf("| 调用/引用边 | %d |\n", edgeCount))
	sb.WriteString("\n")

	// Package summary
	if len(pkgSummary) > 0 {
		sb.WriteString("## 包分布\n\n")
		sb.WriteString("| 包 | 函数 | 变量 | 常量 |\n")
		sb.WriteString("|----|------|------|------|\n")
		for _, p := range pkgSummary {
			sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d |\n", shortPkgName(p.Package), p.FuncCount, p.VarCount, p.ConstCount))
		}
		sb.WriteString("\n")
	}

	// Top risky functions
	if len(risks) > 0 {
		sb.WriteString("## 高风险函数 (Top 5)\n\n")
		sb.WriteString("| 风险 | 函数 | 直接调用者 |\n")
		sb.WriteString("|------|------|------------|\n")
		for _, r := range risks {
			sb.WriteString(fmt.Sprintf("| %s %s | %s | %d |\n", getRiskIcon(r.RiskLevel), r.RiskLevel, display.ShortFuncName(r.Node.Name), r.DirectCallers))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// shortPkgName extracts the last 2 segments of a package path
func shortPkgName(pkg string) string {
	parts := strings.Split(pkg, "/")
	if len(parts) <= 2 {
		return pkg
	}
	return strings.Join(parts[len(parts)-2:], "/")
}


// formatAmbiguousResult returns a formatted message listing candidate functions
// when a function name matches multiple results, asking the AI to retry with a full name.
func (s *Server) formatAmbiguousResult(funcName string, nodes []*graph.Node) string {
	result := fmt.Sprintf("函数名 '%s' 匹配到 %d 个结果，请使用完整函数名重新调用：\n\n", funcName, len(nodes))
	for i, n := range nodes {
		result += fmt.Sprintf("  [%d] %s\n      %s:%d\n", i+1, n.Name, n.File, n.Line)
	}
	result += "\n请使用上述完整函数名（如第一列所示）重新调用此工具。"
	return result
}

func (s *Server) toolImpact(args map[string]interface{}) (string, bool) {
	funcName, ok := args["function"].(string)
	if !ok || funcName == "" {
		return "错误：需要提供函数名称", true
	}

	upstreamDepth := 7
	downstreamDepth := 7

	analyzer := impact.NewAnalyzer(s.db)
	report, err := analyzer.AnalyzeImpact(funcName, upstreamDepth, downstreamDepth)
	if err != nil {
		if strings.Contains(err.Error(), "ambiguous function name") {
			nodes, _ := s.db.FindNodesByPattern(funcName)
			if len(nodes) > 1 {
				return s.formatAmbiguousResult(funcName, nodes), false
			}
		}
		return fmt.Sprintf("错误：%v", err), true
	}

	return s.formatImpactAsTree(report, upstreamDepth, downstreamDepth), false
}

func (s *Server) formatImpactAsTree(report *impact.ImpactReport, upstreamDepth, downstreamDepth int) string {
	var result string

	// For var/const, show referencing functions as flat list (same as CLI)
	isVarConst := report.Target.Kind == graph.NodeKindVar || report.Target.Kind == graph.NodeKindConst

	if isVarConst {
		kindLabel := "变量"
		if report.Target.Kind == graph.NodeKindConst {
			kindLabel = "常量"
		}
		result += fmt.Sprintf("📍 当前%s\n", kindLabel)
		result += fmt.Sprintf("%s  %s:%d\n", display.ShortFuncName(report.Target.Name), report.Target.File, report.Target.Line)
		if report.Target.Signature != "" {
			result += fmt.Sprintf("   类型: %s\n", report.Target.Signature)
		}
		result += "\n"

		if len(report.DirectCallers) > 0 {
			result += fmt.Sprintf("⬆️ 引用此%s的函数 (共 %d 个)\n", kindLabel, len(report.DirectCallers))
			for i, c := range report.DirectCallers {
				prefix := "├──"
				if i == len(report.DirectCallers)-1 {
					prefix = "└──"
				}
				result += fmt.Sprintf("%s %s  %s:%d\n", prefix, display.ShortFuncName(c.Name), c.File, c.Line)
			}
		} else {
			result += fmt.Sprintf("⬆️ 引用此%s的函数\n", kindLabel)
			result += "└── (无)\n"
		}
		return result
	}

	// For functions: build upstream and downstream trees
	upstreamTree, _ := s.db.GetUpstreamCallTree(report.Target.ID, upstreamDepth)
	downstreamTree, _ := s.db.GetDownstreamCallTree(report.Target.ID, downstreamDepth)

	maxWidth := len(display.ShortFuncName(report.Target.Name))
	upstreamMaxDepth := 0
	downstreamMaxDepth := 0
	display.CalcTreeMaxWidth(upstreamTree, &maxWidth, 0, &upstreamMaxDepth)
	display.CalcTreeMaxWidth(downstreamTree, &maxWidth, 0, &downstreamMaxDepth)

	result += "📍 当前函数\n"
	targetMaxDepth := upstreamMaxDepth
	if downstreamMaxDepth > targetMaxDepth {
		targetMaxDepth = downstreamMaxDepth
	}
	targetPadding := maxWidth + targetMaxDepth*4
	result += fmt.Sprintf("%-*s  %s:%d\n", targetPadding, display.ShortFuncName(report.Target.Name), report.Target.File, report.Target.Line)
	if report.Target.Signature != "" {
		result += fmt.Sprintf("   %s\n", display.ShortSignature(report.Target.Signature))
	}
	result += "\n"

	if len(upstreamTree) > 0 {
		result += fmt.Sprintf("⬆️ 调用者 (深度 %d)\n", upstreamDepth)
		result += display.FormatCallTree(upstreamTree, "", maxWidth, upstreamMaxDepth, 0)
	} else {
		result += "⬆️ 调用者\n└── (无)\n"
	}
	result += "\n"

	if len(downstreamTree) > 0 {
		result += fmt.Sprintf("⬇️ 被调用 (深度 %d)\n", downstreamDepth)
		result += display.FormatCallTree(downstreamTree, "", maxWidth, downstreamMaxDepth, 0)
	} else {
		result += "⬇️ 被调用\n└── (无)\n"
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

	// Find the function
	nodes, err := s.db.FindNodesByPattern(funcName)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}
	if len(nodes) == 0 {
		return fmt.Sprintf("未找到函数：%s\n\n💡 提示：如果这是新添加的函数，请运行以下命令更新数据库：\n```bash\ncrag analyze -i -r\n```", funcName), true
	}
	if len(nodes) > 1 {
		return s.formatAmbiguousResult(funcName, nodes), false
	}

	node := nodes[0]
	callTree, err := s.db.GetUpstreamCallTree(node.ID, depth)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	maxWidth := len(display.ShortFuncName(node.Name))
	maxDepth := 0
	display.CalcTreeMaxWidth(callTree, &maxWidth, 0, &maxDepth)

	targetPadding := maxWidth + maxDepth*4
	result := "📍 当前函数\n"
	result += fmt.Sprintf("%-*s  %s:%d\n\n", targetPadding, display.ShortFuncName(node.Name), node.File, node.Line)

	if len(callTree) > 0 {
		result += fmt.Sprintf("⬆️ 调用者 (深度 %d)\n", depth)
		result += display.FormatCallTree(callTree, "", maxWidth, maxDepth, 0)
	} else {
		result += "⬆️ 调用者\n└── (无)\n"
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

	// Find the function
	nodes, err := s.db.FindNodesByPattern(funcName)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}
	if len(nodes) == 0 {
		return fmt.Sprintf("未找到函数：%s\n\n💡 提示：如果这是新添加的函数，请运行以下命令更新数据库：\n```bash\ncrag analyze -i -r\n```", funcName), true
	}
	if len(nodes) > 1 {
		return s.formatAmbiguousResult(funcName, nodes), false
	}

	node := nodes[0]
	callTree, err := s.db.GetDownstreamCallTree(node.ID, depth)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	maxWidth := len(display.ShortFuncName(node.Name))
	maxDepth := 0
	display.CalcTreeMaxWidth(callTree, &maxWidth, 0, &maxDepth)

	targetPadding := maxWidth + maxDepth*4
	result := "📍 当前函数\n"
	result += fmt.Sprintf("%-*s  %s:%d\n\n", targetPadding, display.ShortFuncName(node.Name), node.File, node.Line)

	if len(callTree) > 0 {
		result += fmt.Sprintf("⬇️ 被调用 (深度 %d)\n", depth)
		result += display.FormatCallTree(callTree, "", maxWidth, maxDepth, 0)
	} else {
		result += "⬇️ 被调用\n└── (无)\n"
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

	result := fmt.Sprintf("找到 %d 个匹配", total)
	if total > limit {
		result += fmt.Sprintf("（显示前 %d 个）", limit)
	}
	result += ":\n\n"

	for _, n := range nodes {
		result += fmt.Sprintf("  [%s] %s\n    %s:%d\n", n.Kind, display.ShortFuncName(n.Name), n.File, n.Line)
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

	kind := "func"
	if k, ok := args["kind"].(string); ok && k != "" {
		kind = k
	}

	var nodes []*graph.Node
	var err error
	var kindLabel string
	switch kind {
	case "var":
		nodes, err = s.db.GetAllVars()
		kindLabel = "变量"
	case "const":
		nodes, err = s.db.GetAllConsts()
		kindLabel = "常量"
	case "func":
		nodes, err = s.db.GetAllFunctions()
		kindLabel = "函数"
	case "interface":
		nodes, err = s.db.GetAllInterfaces()
		kindLabel = "接口"
	case "struct":
		nodes, err = s.db.GetAllTypes()
		kindLabel = "结构体"
	default:
		return fmt.Sprintf("未知类型: %s，支持: func/var/const/interface/struct", kind), true
	}
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	if len(nodes) == 0 {
		return fmt.Sprintf("项目中没有%s", kindLabel), false
	}

	total := len(nodes)

	// Apply offset
	if offset >= total {
		return fmt.Sprintf("偏移量 %d 超出范围（共 %d 个%s）", offset, total, kindLabel), false
	}
	if offset > 0 {
		nodes = nodes[offset:]
	}

	// Apply limit
	displayed := len(nodes)
	if limit > 0 && limit < len(nodes) {
		nodes = nodes[:limit]
		displayed = limit
	}

	result := fmt.Sprintf("共 %d 个%s", total, kindLabel)
	if offset > 0 || displayed < total-offset {
		result += fmt.Sprintf("（显示 %d-%d）", offset+1, offset+displayed)
	}
	result += ":\n\n"

	for _, n := range nodes {
		result += fmt.Sprintf("  %s\n    %s:%d\n", display.ShortFuncName(n.Name), n.File, n.Line)
	}

	return result, false
}

func (s *Server) toolImplements(args map[string]interface{}) (string, bool) {
	listAll := false
	if l, ok := args["list"].(bool); ok {
		listAll = l
	}

	if listAll {
		// List all interfaces
		interfaces, err := s.db.GetAllInterfaces()
		if err != nil {
			return fmt.Sprintf("错误：%v", err), true
		}

		if len(interfaces) == 0 {
			return "项目中没有接口定义\n\n💡 提示：请先运行 analyze 命令分析项目", false
		}

		result := fmt.Sprintf("## 项目接口列表 (共 %d 个)\n\n", len(interfaces))
		for _, iface := range interfaces {
			methods := display.ShortSignature(iface.Signature)
			if methods == "" {
				methods = "(空接口)"
			}
			result += fmt.Sprintf("**%s**\n", display.ShortFuncName(iface.Name))
			result += fmt.Sprintf("- 方法: %s\n", methods)
			result += fmt.Sprintf("- 位置: %s:%d\n\n", iface.File, iface.Line)
		}
		return result, false
	}

	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "错误：请提供接口或类型名称，或设置 list=true 列出所有接口", true
	}

	// Try to find as interface first
	interfaces, err := s.db.FindInterfacesByPattern(name)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	if len(interfaces) > 0 {
		// Found interface(s), show implementations
		iface := interfaces[0]
		result := fmt.Sprintf("## 接口: %s\n\n", display.ShortFuncName(iface.Name))
		result += fmt.Sprintf("- 位置: %s:%d\n", iface.File, iface.Line)
		if iface.Signature != "" {
			result += fmt.Sprintf("- 方法: %s\n", display.ShortSignature(iface.Signature))
		}
		result += "\n"

		impls, err := s.db.GetImplementations(iface.ID)
		if err != nil {
			return fmt.Sprintf("错误：%v", err), true
		}

		if len(impls) == 0 {
			result += "没有找到实现此接口的类型\n"
		} else {
			result += fmt.Sprintf("### 实现类型 (共 %d 个)\n\n", len(impls))
			for _, impl := range impls {
				result += fmt.Sprintf("- **%s** - %s:%d\n",
					display.ShortFuncName(impl.Name), impl.File, impl.Line)
			}
		}
		return result, false
	}

	// Try to find as type (struct)
	nodes, err := s.db.FindNodesByPattern(name)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	// Filter to only struct types
	for _, node := range nodes {
		if node.Kind == "struct" {
			result := fmt.Sprintf("## 类型: %s\n\n", display.ShortFuncName(node.Name))
			result += fmt.Sprintf("- 位置: %s:%d\n\n", node.File, node.Line)

			implInterfaces, err := s.db.GetImplementedInterfaces(node.ID)
			if err != nil {
				return fmt.Sprintf("错误：%v", err), true
			}

			if len(implInterfaces) == 0 {
				result += "此类型没有实现任何接口\n"
			} else {
				result += fmt.Sprintf("### 实现的接口 (共 %d 个)\n\n", len(implInterfaces))
				for _, iface := range implInterfaces {
					methods := display.ShortSignature(iface.Signature)
					if methods == "" {
						methods = "(空接口)"
					}
					result += fmt.Sprintf("- **%s** - %s\n", display.ShortFuncName(iface.Name), methods)
					result += fmt.Sprintf("  - %s:%d\n", iface.File, iface.Line)
				}
			}
			return result, false
		}
	}

	return fmt.Sprintf("未找到名为 '%s' 的接口或类型\n\n💡 提示：请先运行 analyze 命令分析项目", name), false
}

func (s *Server) toolRisk(args map[string]interface{}) (string, bool) {
	limit := 20
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	funcName, hasFunc := args["function"].(string)
	if !hasFunc || funcName == "" {
		// Show top risky functions
		risks, err := s.db.GetTopRiskyFunctions(limit)
		if err != nil {
			return fmt.Sprintf("错误：%v", err), true
		}

		if len(risks) == 0 {
			return "项目中没有函数", false
		}

		result := fmt.Sprintf("## 高风险函数排行 (Top %d)\n\n", limit)
		for _, r := range risks {
			riskIcon := getRiskIcon(r.RiskLevel)
			result += fmt.Sprintf("%s **%s** - %s\n", riskIcon, r.RiskLevel, display.ShortFuncName(r.Node.Name))
			result += fmt.Sprintf("   调用者: %d | %s:%d\n\n", r.DirectCallers, r.Node.File, r.Node.Line)
		}
		result += "风险等级: 🔴critical(>=50) 🟠high(>=20) 🟡medium(>=5) 🟢low\n"
		return result, false
	}

	// Analyze specific function
	nodes, err := s.db.FindNodesByPattern(funcName)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	if len(nodes) == 0 {
		return fmt.Sprintf("未找到函数: %s\n\n💡 提示：如果这是新添加的函数，请运行以下命令更新数据库：\n```bash\ncrag analyze -i -r\n```", funcName), true
	}
	if len(nodes) > 1 {
		return s.formatAmbiguousResult(funcName, nodes), false
	}

	node := nodes[0]
	risk, err := s.db.GetRiskScore(node.ID)
	if err != nil {
		return fmt.Sprintf("错误：%v", err), true
	}

	riskIcon := getRiskIcon(risk.RiskLevel)
	result := fmt.Sprintf("## 变更风险分析: %s\n\n", display.ShortFuncName(risk.Node.Name))
	result += fmt.Sprintf("**位置:** %s:%d\n", risk.Node.File, risk.Node.Line)
	if risk.Node.Signature != "" {
		result += fmt.Sprintf("**签名:** `%s`\n", risk.Node.Signature)
	}
	result += "\n"

	result += fmt.Sprintf("### 风险等级: %s %s\n\n", riskIcon, risk.RiskLevel)
	result += fmt.Sprintf("直接调用者: %d\n", risk.DirectCallers)

	result += "\n**建议:**\n"
	switch risk.RiskLevel {
	case "critical":
		result += "- ⚠️ 此函数被大量调用，修改需极其谨慎\n"
		result += "- 建议先运行 impact 工具查看完整影响范围\n"
		result += "- 修改前确保有充分的测试覆盖\n"
	case "high":
		result += "- ⚠️ 此函数调用者较多，修改需谨慎\n"
		result += "- 建议运行 upstream 工具查看调用者\n"
	case "medium":
		result += "- 正常风险，注意检查调用处是否需要同步修改\n"
	case "low":
		result += "- 低风险，影响范围较小，正常修改即可\n"
	}

	return result, false
}

func getRiskIcon(level string) string {
	switch level {
	case "critical":
		return "🔴"
	case "high":
		return "🟠"
	case "medium":
		return "🟡"
	default:
		return "🟢"
	}
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
	if len(nodes) > 1 {
		return s.formatAmbiguousResult(funcName, nodes), false
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
