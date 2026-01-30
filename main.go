package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/zheng/crag/internal/analyzer"
	"github.com/zheng/crag/internal/export"
	"github.com/zheng/crag/internal/graph"
	"github.com/zheng/crag/internal/impact"
	"github.com/zheng/crag/internal/mcp"
	"github.com/zheng/crag/internal/storage"
	"github.com/zheng/crag/internal/watcher"
	"github.com/zheng/crag/internal/web"
)

var (
	dbPath string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "crag",
		Short: "Code RAG - Go代码调用图分析工具",
		Long: `crag 是一个 Go 代码静态分析工具，用于构建函数调用图谱，
帮助追踪代码变更的影响范围，减少 AI 编码时的漏改问题。`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&dbPath, "db", "d", ".crag.db", "数据库文件路径")

	// Add commands
	rootCmd.AddCommand(analyzeCmd())
	rootCmd.AddCommand(upstreamCmd())
	rootCmd.AddCommand(downstreamCmd())
	rootCmd.AddCommand(impactCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(exportCmd())
	rootCmd.AddCommand(mcpCmd())
	rootCmd.AddCommand(watchCmd())
	rootCmd.AddCommand(viewCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func analyzeCmd() *cobra.Command {
	var outputPath string
	var incremental bool
	var gitBase string
	var remote bool

	cmd := &cobra.Command{
		Use:   "analyze [project-path]",
		Short: "分析 Go 项目并构建调用图",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectPath := "."
			if len(args) > 0 {
				projectPath = args[0]
			}

			if outputPath != "" {
				dbPath = outputPath
			}

			// Incremental mode: detect changed files
			var changedPackages []string
			if incremental {
				// 如果启用 remote 模式，自动获取远程分支作为 base
				if remote {
					remoteBranch, err := analyzer.GetRemoteTrackingBranch(projectPath)
					if err != nil {
						fmt.Printf("警告: 无法获取远程分支: %v，将使用默认 HEAD\n", err)
					} else {
						gitBase = remoteBranch
						fmt.Printf("对比远程分支: %s\n", remoteBranch)
					}
				}

				fmt.Println("检测 git 变更...")
				changes, err := analyzer.GetGitChanges(projectPath, gitBase)
				if err != nil {
					fmt.Printf("警告: 无法获取 git 变更，将执行全量分析: %v\n", err)
					incremental = false
				} else if !changes.HasChanges() {
					fmt.Println("没有检测到 Go 文件变更，跳过分析")
					return nil
				} else {
					fmt.Printf("检测到 %d 个变更文件，涉及 %d 个包:\n", len(changes.ChangedFiles), len(changes.ChangedPackages))
					for _, f := range changes.ChangedFiles {
						fmt.Printf("  - %s\n", f)
					}
					changedPackages = changes.ChangedPackages
				}
			}

			// Load packages
			pkgs, err := analyzer.LoadPackages(projectPath)
			if err != nil {
				return fmt.Errorf("加载包失败: %w", err)
			}

			// Filter packages with source
			pkgs = analyzer.FilterMainPackages(pkgs)
			if len(pkgs) == 0 {
				return fmt.Errorf("未找到有效的 Go 包")
			}

			// Convert changed package dirs to full package paths for incremental mode
			if incremental && len(changedPackages) > 0 {
				fullPkgPaths := make([]string, 0, len(changedPackages))
				for _, relativePath := range changedPackages {
					// Remove leading "./" from relative path
					// e.g., "./controllers/chatroom/pk" -> "controllers/chatroom/pk"
					suffix := strings.TrimPrefix(relativePath, "./")
					if suffix == "" {
						suffix = "."
					}

					// Find matching package by checking if package path ends with the relative path
					for _, pkg := range pkgs {
						if pkg.PkgPath != "" {
							// Check if package path ends with the relative path
							// e.g., "github.com/xxx/controllers/chatroom/pk" ends with "controllers/chatroom/pk"
							if strings.HasSuffix(pkg.PkgPath, "/"+suffix) || strings.HasSuffix(pkg.PkgPath, suffix) {
								fullPkgPaths = append(fullPkgPaths, pkg.PkgPath)
								break
							}
						}
					}
				}
				changedPackages = fullPkgPaths
				fmt.Printf("转换为完整包路径: %v\n", changedPackages)
			}

			// Build SSA
			prog, _ := analyzer.BuildSSA(pkgs)

			// Build call graph
			cg, err := analyzer.BuildCallGraph(prog)
			if err != nil {
				return fmt.Errorf("构建调用图失败: %w", err)
			}

			// Open database
			db, err := storage.Open(dbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			// Incremental mode: only delete changed packages' data
			if incremental && len(changedPackages) > 0 {
				fmt.Printf("增量模式：删除 %d 个变更包的旧数据...\n", len(changedPackages))
				deletedCount, err := db.DeleteNodesByPackage(changedPackages)
				if err != nil {
					fmt.Printf("警告：删除旧数据失败: %v，将执行全量重建\n", err)
					if err := db.Clear(); err != nil {
						return fmt.Errorf("清空数据库失败: %w", err)
					}
				} else {
					fmt.Printf("已删除 %d 个旧节点\n", deletedCount)
					// Clean up orphan edges
					orphanCount, _ := db.DeleteOrphanEdges()
					if orphanCount > 0 {
						fmt.Printf("清理 %d 条孤立边\n", orphanCount)
					}
				}
			} else {
				// Full rebuild mode
				if err := db.Clear(); err != nil {
					return fmt.Errorf("清空数据库失败: %w", err)
				}
			}

			// Build and store graph
			builder := graph.NewBuilder(
				prog.Fset,
				pkgs,
				projectPath,
				db.InsertNode,
				db.InsertEdge,
			)

			// Set target packages for incremental mode
			if incremental && len(changedPackages) > 0 {
				builder.SetTargetPackages(changedPackages)
				fmt.Printf("增量模式：仅插入变更包的节点\n")
			}

			if err := builder.Build(cg); err != nil {
				return fmt.Errorf("构建图失败: %w", err)
			}

			// Show final stats (only these 3 lines)
			nodeCount, edgeCount, _ := db.GetStats()
			fmt.Printf("写入数据库: %s\n", dbPath)
			fmt.Printf("完成! 已存储 %d 个函数节点\n", builder.GetNodeCount())
			fmt.Printf("数据库总计: %d 节点, %d 边\n", nodeCount, edgeCount)

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "输出数据库路径")
	cmd.Flags().BoolVarP(&incremental, "incremental", "i", false, "增量分析模式 (只分析 git 变更)")
	cmd.Flags().StringVar(&gitBase, "base", "HEAD", "git 比较基准 (默认 HEAD，即未提交的变更)")
	cmd.Flags().BoolVarP(&remote, "remote", "r", false, "与远程同分支对比 (origin/<当前分支>)")

	return cmd
}

func upstreamCmd() *cobra.Command {
	var depth int
	var format string

	cmd := &cobra.Command{
		Use:   "upstream <function-name>",
		Short: "查询函数的上游调用者",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			funcName := args[0]

			db, err := storage.Open(dbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			a := impact.NewAnalyzer(db)
			report, err := a.AnalyzeImpact(funcName, depth, 1) // Only upstream
			if err != nil {
				// Check if it's an ambiguous function name error
				if strings.Contains(err.Error(), "ambiguous function name") {
					nodes, _ := db.FindNodesByPattern(funcName)
					if len(nodes) > 1 {
						fmt.Println("找到多个匹配的函数，请选择:")
						for i, n := range nodes {
							fmt.Printf("  [%d] %s\n      %s:%d\n", i+1, shortFuncName(n.Name), n.File, n.Line)
						}
						fmt.Print("\n请输入序号 [1-" + fmt.Sprint(len(nodes)) + "]: ")

						var choice int
						if _, err := fmt.Scanf("%d", &choice); err != nil || choice < 1 || choice > len(nodes) {
							return fmt.Errorf("无效的选择")
						}

						selectedNode := nodes[choice-1]
						report, err = a.AnalyzeImpact(selectedNode.Name, depth, 1)
						if err != nil {
							return err
						}
					} else {
						return err
					}
				} else {
					return err
				}
			}

			switch format {
			case "json":
				return outputJSON(report.DirectCallers)
			case "markdown":
				fmt.Printf("## 上游调用者: %s\n\n", report.Target.Name)
				if len(report.DirectCallers) == 0 && len(report.IndirectCallers) == 0 {
					fmt.Println("_无上游调用者_")
				} else {
					fmt.Println("| 函数 | 文件 | 行号 |")
					fmt.Println("|------|------|------|")
					for _, c := range report.DirectCallers {
						fmt.Printf("| %s | %s | %d |\n", c.Name, c.File, c.Line)
					}
					for _, c := range report.IndirectCallers {
						fmt.Printf("| %s | %s | %d |\n", c.Name, c.File, c.Line)
					}
				}
			default:
				// Get call tree
				callTree, err := db.GetUpstreamCallTree(report.Target.ID, depth)
				if err != nil {
					return fmt.Errorf("获取调用树失败: %w", err)
				}

				// Calculate max width and depth for alignment
				maxWidth := len(shortFuncName(report.Target.Name))
				maxDepth := 0
				calcTreeMaxWidth(callTree, &maxWidth, 0, &maxDepth)

				fmt.Println("📍 当前函数")
				// Target function gets extra padding to align with deepest tree item
				targetPadding := maxWidth + maxDepth*4
				fmt.Printf("%-*s  %s:%d\n\n", targetPadding, shortFuncName(report.Target.Name), shortFilePath(report.Target.File), report.Target.Line)

				if len(callTree) > 0 {
					fmt.Printf("⬆️ 调用者 (深度 %d)\n", depth)
					printCallTree(callTree, "", true, maxWidth, maxDepth, 0)
				} else {
					fmt.Println("⬆️ 调用者")
					fmt.Println("└── (无)")
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&depth, "depth", 7, "递归深度 (0=无限)")
	cmd.Flags().StringVar(&format, "format", "text", "输出格式 (text/json/markdown)")

	return cmd
}

func downstreamCmd() *cobra.Command {
	var depth int
	var format string

	cmd := &cobra.Command{
		Use:   "downstream <function-name>",
		Short: "查询函数的下游依赖",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			funcName := args[0]

			db, err := storage.Open(dbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			a := impact.NewAnalyzer(db)
			report, err := a.AnalyzeImpact(funcName, 1, depth) // Only downstream
			if err != nil {
				// Check if it's an ambiguous function name error
				if strings.Contains(err.Error(), "ambiguous function name") {
					nodes, _ := db.FindNodesByPattern(funcName)
					if len(nodes) > 1 {
						fmt.Println("找到多个匹配的函数，请选择:")
						for i, n := range nodes {
							fmt.Printf("  [%d] %s\n      %s:%d\n", i+1, shortFuncName(n.Name), n.File, n.Line)
						}
						fmt.Print("\n请输入序号 [1-" + fmt.Sprint(len(nodes)) + "]: ")

						var choice int
						if _, err := fmt.Scanf("%d", &choice); err != nil || choice < 1 || choice > len(nodes) {
							return fmt.Errorf("无效的选择")
						}

						selectedNode := nodes[choice-1]
						report, err = a.AnalyzeImpact(selectedNode.Name, 1, depth)
						if err != nil {
							return err
						}
					} else {
						return err
					}
				} else {
					return err
				}
			}

			switch format {
			case "json":
				return outputJSON(report.DirectCallees)
			case "markdown":
				fmt.Printf("## 下游依赖: %s\n\n", report.Target.Name)
				if len(report.DirectCallees) == 0 && len(report.IndirectCallees) == 0 {
					fmt.Println("_无下游依赖_")
				} else {
					fmt.Println("| 函数 | 文件 | 行号 |")
					fmt.Println("|------|------|------|")
					for _, c := range report.DirectCallees {
						fmt.Printf("| %s | %s | %d |\n", c.Name, c.File, c.Line)
					}
					for _, c := range report.IndirectCallees {
						fmt.Printf("| %s | %s | %d |\n", c.Name, c.File, c.Line)
					}
				}
			default:
				// Get call tree
				callTree, err := db.GetDownstreamCallTree(report.Target.ID, depth)
				if err != nil {
					return fmt.Errorf("获取调用树失败: %w", err)
				}

				// Calculate max width and depth for alignment
				maxWidth := len(shortFuncName(report.Target.Name))
				maxDepth := 0
				calcTreeMaxWidth(callTree, &maxWidth, 0, &maxDepth)

				fmt.Println("📍 当前函数")
				// Target function gets extra padding to align with deepest tree item
				targetPadding := maxWidth + maxDepth*4
				fmt.Printf("%-*s  %s:%d\n\n", targetPadding, shortFuncName(report.Target.Name), shortFilePath(report.Target.File), report.Target.Line)

				if len(callTree) > 0 {
					fmt.Printf("⬇️ 被调用 (深度 %d)\n", depth)
					printCallTree(callTree, "", false, maxWidth, maxDepth, 0)
				} else {
					fmt.Println("⬇️ 被调用")
					fmt.Println("└── (无)")
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&depth, "depth", 7, "递归深度 (0=无限)")
	cmd.Flags().StringVar(&format, "format", "text", "输出格式 (text/json/markdown)")

	return cmd
}

func impactCmd() *cobra.Command {
	var upstreamDepth int
	var downstreamDepth int
	var format string

	cmd := &cobra.Command{
		Use:   "impact <function-name>",
		Short: "分析函数变更的影响范围",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			funcName := args[0]

			db, err := storage.Open(dbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			a := impact.NewAnalyzer(db)
			report, err := a.AnalyzeImpact(funcName, upstreamDepth, downstreamDepth)
			if err != nil {
				// Check if it's an ambiguous function name error
				if strings.Contains(err.Error(), "ambiguous function name") {
					// Extract matches and show selection
					nodes, _ := db.FindNodesByPattern(funcName)
					if len(nodes) > 1 {
						fmt.Println("找到多个匹配的函数，请选择:")
						for i, n := range nodes {
							fmt.Printf("  [%d] %s\n      %s:%d\n", i+1, shortFuncName(n.Name), n.File, n.Line)
						}
						fmt.Print("\n请输入序号 [1-" + fmt.Sprint(len(nodes)) + "]: ")

						var choice int
						if _, err := fmt.Scanf("%d", &choice); err != nil || choice < 1 || choice > len(nodes) {
							return fmt.Errorf("无效的选择")
						}

						// Retry with selected node
						selectedNode := nodes[choice-1]
						report, err = a.AnalyzeImpact(selectedNode.Name, upstreamDepth, downstreamDepth)
						if err != nil {
							return err
						}
					} else {
						return err
					}
				} else {
					return err
				}
			}

			switch format {
			case "json":
				return outputJSON(report)
			case "markdown":
				fmt.Print(report.FormatMarkdown())
			default:
				// Get call trees
				upstreamTree, err := db.GetUpstreamCallTree(report.Target.ID, upstreamDepth)
				if err != nil {
					return fmt.Errorf("获取上游调用树失败: %w", err)
				}
				downstreamTree, err := db.GetDownstreamCallTree(report.Target.ID, downstreamDepth)
				if err != nil {
					return fmt.Errorf("获取下游调用树失败: %w", err)
				}

				// Calculate max width and depth for alignment
				maxWidth := len(shortFuncName(report.Target.Name))
				upstreamMaxDepth := 0
				downstreamMaxDepth := 0
				calcTreeMaxWidth(upstreamTree, &maxWidth, 0, &upstreamMaxDepth)
				calcTreeMaxWidth(downstreamTree, &maxWidth, 0, &downstreamMaxDepth)

				// Print target function
				fmt.Println("📍 当前函数")
				// Use the larger maxDepth for target function alignment
				targetMaxDepth := upstreamMaxDepth
				if downstreamMaxDepth > targetMaxDepth {
					targetMaxDepth = downstreamMaxDepth
				}
				targetPadding := maxWidth + targetMaxDepth*4
				fmt.Printf("%-*s  %s:%d\n", targetPadding, shortFuncName(report.Target.Name), shortFilePath(report.Target.File), report.Target.Line)
				if report.Target.Signature != "" {
					fmt.Printf("   %s\n", shortSignature(report.Target.Signature))
				}
				fmt.Println()

				// Print upstream callers
				if len(upstreamTree) > 0 {
					fmt.Printf("⬆️ 调用者 (深度 %d)\n", upstreamDepth)
					printCallTree(upstreamTree, "", true, maxWidth, upstreamMaxDepth, 0)
				} else {
					fmt.Println("⬆️ 调用者")
					fmt.Println("└── (无)")
				}
				fmt.Println()

				// Print downstream callees
				if len(downstreamTree) > 0 {
					fmt.Printf("⬇️ 被调用 (深度 %d)\n", downstreamDepth)
					printCallTree(downstreamTree, "", false, maxWidth, downstreamMaxDepth, 0)
				} else {
					fmt.Println("⬇️ 被调用")
					fmt.Println("└── (无)")
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&upstreamDepth, "upstream-depth", 7, "上游递归深度")
	cmd.Flags().IntVar(&downstreamDepth, "downstream-depth", 7, "下游递归深度")
	cmd.Flags().StringVar(&format, "format", "text", "输出格式 (text/json/markdown)")

	return cmd
}

func listCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有函数",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.Open(dbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			funcs, err := db.GetAllFunctions()
			if err != nil {
				return fmt.Errorf("查询失败: %w", err)
			}

			fmt.Printf("共 %d 个函数:\n\n", len(funcs))

			count := 0
			for _, f := range funcs {
				if limit > 0 && count >= limit {
					fmt.Printf("... 还有 %d 个函数\n", len(funcs)-limit)
					break
				}
				fmt.Printf("  %s\n    %s:%d\n", f.Name, f.File, f.Line)
				count++
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "限制显示数量 (0=全部)")

	return cmd
}

func searchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <pattern>",
		Short: "搜索函数",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pattern := args[0]

			db, err := storage.Open(dbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			funcs, err := db.FindNodesByPattern(pattern)
			if err != nil {
				return fmt.Errorf("搜索失败: %w", err)
			}

			if len(funcs) == 0 {
				fmt.Println("未找到匹配的函数")
				return nil
			}

			fmt.Printf("找到 %d 个匹配:\n\n", len(funcs))
			for _, f := range funcs {
				fmt.Printf("  %s\n    %s:%d\n", f.Name, f.File, f.Line)
			}

			return nil
		},
	}

	return cmd
}

func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// shortFuncName simplifies a fully qualified function name
// e.g., "(*github.com/foo/bar/pkg.Type).Method" -> "(*pkg.Type).Method"
func shortFuncName(fullName string) string {
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

// shortFilePath returns the file path as-is (already relative to project root)
func shortFilePath(fullPath string) string {
	return fullPath
}

// shortSignature simplifies package paths in a function signature
// e.g., "func(db *github.com/jinzhu/gorm.DB) error" -> "func(db *gorm.DB) error"
func shortSignature(sig string) string {
	// Find and replace all package paths (anything with / before a .)
	// Pattern: something/something/pkg.Type -> pkg.Type
	result := sig
	for {
		// Find a package path pattern: xxx/yyy/pkg.
		start := -1
		for i := 0; i < len(result); i++ {
			if result[i] == '/' {
				// Found a slash, look backwards to find the start
				start = i
				for j := i - 1; j >= 0; j-- {
					c := result[j]
					if c == ' ' || c == '*' || c == '(' || c == '[' || c == ',' {
						start = j + 1
						break
					}
					if j == 0 {
						start = 0
					}
				}
				break
			}
		}
		if start == -1 {
			break
		}

		// Find the last / before the next space, ), or end
		lastSlash := -1
		for i := start; i < len(result); i++ {
			if result[i] == '/' {
				lastSlash = i
			}
			if result[i] == ' ' || result[i] == ')' || result[i] == ',' || result[i] == ']' {
				break
			}
		}

		if lastSlash > start {
			// Replace from start to lastSlash+1 with empty
			result = result[:start] + result[lastSlash+1:]
		} else {
			break
		}
	}
	return result
}

// calcTreeMaxWidth calculates the maximum function name width and depth for alignment in the call tree
func calcTreeMaxWidth(tree []*storage.CallTreeNode, maxWidth *int, currentDepth int, maxDepth *int) {
	if currentDepth > *maxDepth {
		*maxDepth = currentDepth
	}
	for _, node := range tree {
		w := len(shortFuncName(node.Node.Name))
		if w > *maxWidth {
			*maxWidth = w
		}
		if len(node.Children) > 0 {
			calcTreeMaxWidth(node.Children, maxWidth, currentDepth+1, maxDepth)
		}
	}
}

// printCallTree prints the call tree with proper indentation (function name first, then path)
// maxDepth is the maximum depth of the tree, currentDepth is the current level (0-indexed)
// maxWidth is the maximum function name width
func printCallTree(tree []*storage.CallTreeNode, indent string, isUpstream bool, maxWidth int, maxDepth int, currentDepth int) {
	for i, node := range tree {
		isLast := i == len(tree)-1
		prefix := "├──"
		if isLast {
			prefix = "└──"
		}

		funcName := shortFuncName(node.Node.Name)
		loc := fmt.Sprintf("%s:%d", shortFilePath(node.Node.File), node.Node.Line)
		// Adjust padding to account for depth: deeper items have more indent, so need less funcName padding
		// Total column width = (maxDepth+1)*4 + maxWidth, current prefix width = (currentDepth+1)*4
		// So funcName padding = maxWidth + (maxDepth - currentDepth)*4
		padding := maxWidth + (maxDepth-currentDepth)*4
		fmt.Printf("%s%s %-*s  %s\n", indent, prefix, padding, funcName, loc)

		if len(node.Children) > 0 {
			childIndent := indent + "│   "
			if isLast {
				childIndent = indent + "    "
			}
			printCallTree(node.Children, childIndent, isUpstream, maxWidth, maxDepth, currentDepth+1)
		}
	}
}

func exportCmd() *cobra.Command {
	var outputFile string
	var incremental bool
	var gitBase string
	var noMermaid bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "导出 RAG 文档",
		Long:  "导出完整的项目调用图谱文档（Markdown 格式），可作为 AI 编码上下文",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.Open(dbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			exporter := export.NewExporter(db)
			opts := export.DefaultExportOptions()
			opts.IncludeMermaid = !noMermaid

			// Determine output writer
			var w *os.File
			if outputFile == "" || outputFile == "-" {
				w = os.Stdout
			} else {
				w, err = os.Create(outputFile)
				if err != nil {
					return fmt.Errorf("创建输出文件失败: %w", err)
				}
				defer w.Close()
			}

			// Incremental mode
			if incremental {
				// Get current working directory for git operations
				cwd, _ := os.Getwd()
				changes, err := analyzer.GetGitChanges(cwd, gitBase)
				if err != nil {
					return fmt.Errorf("获取 git 变更失败: %w", err)
				}

				if !changes.HasChanges() {
					fmt.Fprintln(os.Stderr, "没有检测到变更")
					return nil
				}

				fmt.Fprintf(os.Stderr, "检测到 %d 个变更文件\n", len(changes.ChangedFiles))
				return exporter.ExportIncremental(w, changes.ChangedPackages, opts)
			}

			// Full export
			return exporter.Export(w, opts)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "输出文件路径 (默认输出到 stdout)")
	cmd.Flags().BoolVarP(&incremental, "incremental", "i", false, "增量导出 (只输出 git 变更部分)")
	cmd.Flags().StringVar(&gitBase, "base", "HEAD", "git 比较基准")
	cmd.Flags().BoolVar(&noMermaid, "no-mermaid", false, "不生成 Mermaid 图表")

	return cmd
}

func mcpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "启动 MCP (Model Context Protocol) 服务器",
		Long: `启动 MCP 服务器，允许 AI 助手（如 Cursor、Claude）直接查询代码调用图。

MCP 工具包括：
  - impact: 分析函数变更的影响范围
  - upstream: 查询上游调用者
  - downstream: 查询下游被调用者
  - search: 搜索函数
  - list: 列出所有函数`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.Open(dbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			server := mcp.NewServer(db)
			return server.Run()
		},
	}

	return cmd
}

func watchCmd() *cobra.Command {
	var debounceMs int

	cmd := &cobra.Command{
		Use:   "watch [project-path]",
		Short: "监控文件变更并自动更新调用图",
		Long: `启动 watch 模式，监控项目中的 Go 文件变更。
当检测到文件变更时，自动重新分析并更新调用图数据库。

特性：
  - 自动递归监控所有目录
  - 防抖处理，避免频繁触发分析
  - 忽略测试文件、隐藏目录、vendor、_test.go 等

示例：
  crag watch .              # 监控当前目录
  crag watch . -o .crag.db  # 指定数据库路径
  crag watch . --debounce 1000  # 设置 1 秒防抖延迟`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectPath := "."
			if len(args) > 0 {
				projectPath = args[0]
			}

			// First run initial analysis
			fmt.Println("执行初始分析...")
			nodeCount, edgeCount, err := runInitialAnalysis(projectPath, dbPath)
			if err != nil {
				return fmt.Errorf("初始分析失败: %w", err)
			}
			fmt.Printf("初始分析完成: %d 节点, %d 边\n", nodeCount, edgeCount)

			// Create watcher
			fmt.Printf("\n开始监控目录: %s\n", projectPath)
			fmt.Printf("数据库路径: %s\n", dbPath)
			fmt.Printf("防抖延迟: %dms\n", debounceMs)
			fmt.Println("\n按 Ctrl+C 停止...")
			fmt.Println()

			w, err := watcher.New(
				projectPath,
				dbPath,
				watcher.WithDebounceDelay(time.Duration(debounceMs)*time.Millisecond),
				watcher.WithOnAnalysisStart(func() {
					fmt.Printf("[%s] 检测到变更，开始分析...\n", time.Now().Format("15:04:05"))
				}),
				watcher.WithOnAnalysisDone(func(nodes, edges int64, duration time.Duration) {
					fmt.Printf("[%s] 分析完成: %d 节点, %d 边 (耗时 %v)\n",
						time.Now().Format("15:04:05"), nodes, edges, duration.Round(time.Millisecond))
				}),
				watcher.WithOnError(func(err error) {
					fmt.Fprintf(os.Stderr, "[%s] 错误: %v\n", time.Now().Format("15:04:05"), err)
				}),
			)
			if err != nil {
				return fmt.Errorf("创建监控器失败: %w", err)
			}

			w.Start()
			defer w.Stop()

			// Wait for interrupt signal
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh

			fmt.Println("\n停止监控...")
			return nil
		},
	}

	cmd.Flags().IntVar(&debounceMs, "debounce", 500, "防抖延迟（毫秒）")

	return cmd
}

// runInitialAnalysis performs the initial code analysis
func runInitialAnalysis(projectPath, dbPath string) (nodeCount, edgeCount int64, err error) {
	// Load packages
	pkgs, err := analyzer.LoadPackages(projectPath)
	if err != nil {
		return 0, 0, fmt.Errorf("加载包失败: %w", err)
	}

	// Filter packages with source
	pkgs = analyzer.FilterMainPackages(pkgs)
	if len(pkgs) == 0 {
		return 0, 0, fmt.Errorf("未找到有效的 Go 包")
	}

	// Build SSA
	prog, _ := analyzer.BuildSSA(pkgs)

	// Build call graph
	cg, err := analyzer.BuildCallGraph(prog)
	if err != nil {
		return 0, 0, fmt.Errorf("构建调用图失败: %w", err)
	}

	// Open database
	db, err := storage.Open(dbPath)
	if err != nil {
		return 0, 0, fmt.Errorf("打开数据库失败: %w", err)
	}
	defer db.Close()

	// Clear existing data
	if err := db.Clear(); err != nil {
		return 0, 0, fmt.Errorf("清空数据库失败: %w", err)
	}

	// Build and store graph
	builder := graph.NewBuilder(
		prog.Fset,
		pkgs,
		projectPath,
		db.InsertNode,
		db.InsertEdge,
	)

	if err := builder.Build(cg); err != nil {
		return 0, 0, fmt.Errorf("构建图失败: %w", err)
	}

	nodeCount, edgeCount, _ = db.GetStats()
	return nodeCount, edgeCount, nil
}

func viewCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "view",
		Short: "启动 Web UI 可视化调用图",
		Long: `启动一个本地 Web 服务器，提供交互式的调用图可视化界面。

特性：
  - 交互式力导向图（缩放、拖拽、点击）
  - 函数搜索和过滤
  - 影响分析（双击节点高亮上下游）
  - 节点详情面板

示例：
  crag view              # 使用默认端口 9998
  crag view -p 3000      # 指定端口
  crag view -d my.db     # 指定数据库`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.Open(dbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			server := web.NewServer(db, port)
			return server.Run()
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 9998, "服务器端口")

	return cmd
}
