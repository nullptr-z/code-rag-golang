package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zheng/crag/internal/impact"
	"github.com/zheng/crag/internal/storage"
)

func upstreamCmd() *cobra.Command {
	var depth int
	var format string
	var selectN int

	cmd := &cobra.Command{
		Use:   "upstream <function-name>",
		Short: "查询函数的上游调用者",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			funcName := args[0]

			db, err := storage.Open(DbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			a := impact.NewAnalyzer(db)
			report, err := a.AnalyzeImpact(funcName, depth, 1)
			if err != nil {
				if strings.Contains(err.Error(), "ambiguous function name") {
					nodes, _ := db.FindNodesByPattern(funcName)
					if len(nodes) > 1 {
						if selectN >= 1 && selectN <= len(nodes) {
							selectedNode := nodes[selectN-1]
							report, err = a.AnalyzeImpact(selectedNode.Name, depth, 1)
							if err != nil {
								return err
							}
						} else {
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
				callTree, err := db.GetUpstreamCallTree(report.Target.ID, depth)
				if err != nil {
					return fmt.Errorf("获取调用树失败: %w", err)
				}

				maxWidth := len(shortFuncName(report.Target.Name))
				maxDepth := 0
				calcTreeMaxWidth(callTree, &maxWidth, 0, &maxDepth)

				fmt.Println("📍 当前函数")
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
	cmd.Flags().IntVar(&selectN, "select", 0, "当匹配到多个函数时，直接选择第N个（跳过交互提示）")

	return cmd
}

func downstreamCmd() *cobra.Command {
	var depth int
	var format string
	var selectN int

	cmd := &cobra.Command{
		Use:   "downstream <function-name>",
		Short: "查询函数的下游依赖",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			funcName := args[0]

			db, err := storage.Open(DbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			a := impact.NewAnalyzer(db)
			report, err := a.AnalyzeImpact(funcName, 1, depth)
			if err != nil {
				if strings.Contains(err.Error(), "ambiguous function name") {
					nodes, _ := db.FindNodesByPattern(funcName)
					if len(nodes) > 1 {
						if selectN >= 1 && selectN <= len(nodes) {
							selectedNode := nodes[selectN-1]
							report, err = a.AnalyzeImpact(selectedNode.Name, 1, depth)
							if err != nil {
								return err
							}
						} else {
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
				callTree, err := db.GetDownstreamCallTree(report.Target.ID, depth)
				if err != nil {
					return fmt.Errorf("获取调用树失败: %w", err)
				}

				maxWidth := len(shortFuncName(report.Target.Name))
				maxDepth := 0
				calcTreeMaxWidth(callTree, &maxWidth, 0, &maxDepth)

				fmt.Println("📍 当前函数")
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
	cmd.Flags().IntVar(&selectN, "select", 0, "当匹配到多个函数时，直接选择第N个（跳过交互提示）")

	return cmd
}

func impactCmd() *cobra.Command {
	var upstreamDepth int
	var downstreamDepth int
	var format string
	var selectN int

	cmd := &cobra.Command{
		Use:   "impact <function-name>",
		Short: "分析函数变更的影响范围",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			funcName := args[0]

			db, err := storage.Open(DbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			a := impact.NewAnalyzer(db)
			report, err := a.AnalyzeImpact(funcName, upstreamDepth, downstreamDepth)
			if err != nil {
				if strings.Contains(err.Error(), "ambiguous function name") {
					nodes, _ := db.FindNodesByPattern(funcName)
					if len(nodes) > 1 {
						if selectN >= 1 && selectN <= len(nodes) {
							selectedNode := nodes[selectN-1]
							report, err = a.AnalyzeImpact(selectedNode.Name, upstreamDepth, downstreamDepth)
							if err != nil {
								return err
							}
						} else {
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
							report, err = a.AnalyzeImpact(selectedNode.Name, upstreamDepth, downstreamDepth)
							if err != nil {
								return err
							}
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
				upstreamTree, err := db.GetUpstreamCallTree(report.Target.ID, upstreamDepth)
				if err != nil {
					return fmt.Errorf("获取上游调用树失败: %w", err)
				}
				downstreamTree, err := db.GetDownstreamCallTree(report.Target.ID, downstreamDepth)
				if err != nil {
					return fmt.Errorf("获取下游调用树失败: %w", err)
				}

				maxWidth := len(shortFuncName(report.Target.Name))
				upstreamMaxDepth := 0
				downstreamMaxDepth := 0
				calcTreeMaxWidth(upstreamTree, &maxWidth, 0, &upstreamMaxDepth)
				calcTreeMaxWidth(downstreamTree, &maxWidth, 0, &downstreamMaxDepth)

				fmt.Println("📍 当前函数")
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

				if len(upstreamTree) > 0 {
					fmt.Printf("⬆️ 调用者 (深度 %d)\n", upstreamDepth)
					printCallTree(upstreamTree, "", true, maxWidth, upstreamMaxDepth, 0)
				} else {
					fmt.Println("⬆️ 调用者")
					fmt.Println("└── (无)")
				}
				fmt.Println()

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
	cmd.Flags().IntVar(&selectN, "select", 0, "当匹配到多个函数时，直接选择第N个（跳过交互提示）")

	return cmd
}

func listCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有函数",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.Open(DbPath)
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

			db, err := storage.Open(DbPath)
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
