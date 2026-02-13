package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zheng/crag/internal/analyzer"
	"github.com/zheng/crag/internal/graph"
	"github.com/zheng/crag/internal/storage"
)

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
				DbPath = outputPath
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
					suffix := strings.TrimPrefix(relativePath, "./")
					if suffix == "" {
						suffix = "."
					}

					for _, pkg := range pkgs {
						if pkg.PkgPath != "" {
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
			db, err := storage.Open(DbPath)
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
					orphanCount, _ := db.DeleteOrphanEdges()
					if orphanCount > 0 {
						fmt.Printf("清理 %d 条孤立边\n", orphanCount)
					}
				}
			} else {
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

			if incremental && len(changedPackages) > 0 {
				builder.SetTargetPackages(changedPackages)
				fmt.Printf("增量模式：仅插入变更包的节点\n")
			}

			if err := builder.Build(cg); err != nil {
				return fmt.Errorf("构建图失败: %w", err)
			}

			// Build interface implementation graph
			interfaceAnalyzer := analyzer.NewInterfaceAnalyzer(pkgs, projectPath)
			ifaceCount, typeCount, implCount, err := interfaceAnalyzer.BuildInterfaceGraph(
				db.InsertNode,
				db.InsertEdge,
			)
			if err != nil {
				fmt.Printf("警告: 接口分析失败: %v\n", err)
			} else if ifaceCount > 0 || typeCount > 0 {
				fmt.Printf("接口分析: %d 个接口, %d 个类型, %d 个实现关系\n", ifaceCount, typeCount, implCount)
			}

			// Build var/const reference graph
			varConstAnalyzer := analyzer.NewVarConstAnalyzer(pkgs, projectPath)
			if incremental && len(changedPackages) > 0 {
				varConstAnalyzer.SetTargetPackages(changedPackages)
			}
			varCount, constCount, refCount, err := varConstAnalyzer.BuildVarConstGraph(
				db.InsertNode,
				db.InsertEdge,
				builder.GetNodeMap(),
			)
			if err != nil {
				fmt.Printf("警告: 变量/常量分析失败: %v\n", err)
			} else if varCount > 0 || constCount > 0 {
				fmt.Printf("变量/常量分析: %d 个变量, %d 个常量, %d 个引用关系\n", varCount, constCount, refCount)
			}

			nodeCount, edgeCount, _ := db.GetStats()
			fmt.Printf("写入数据库: %s\n", DbPath)
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
