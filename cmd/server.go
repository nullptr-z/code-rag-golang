package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/zheng/crag/internal/analyzer"
	"github.com/zheng/crag/internal/graph"
	"github.com/zheng/crag/internal/mcp"
	"github.com/zheng/crag/internal/storage"
	"github.com/zheng/crag/internal/watcher"
	"github.com/zheng/crag/internal/web"
)

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
			db, err := storage.Open(DbPath)
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

			fmt.Println("执行初始分析...")
			nodeCount, edgeCount, err := runInitialAnalysis(projectPath, DbPath)
			if err != nil {
				return fmt.Errorf("初始分析失败: %w", err)
			}
			fmt.Printf("初始分析完成: %d 节点, %d 边\n", nodeCount, edgeCount)

			fmt.Printf("\n开始监控目录: %s\n", projectPath)
			fmt.Printf("数据库路径: %s\n", DbPath)
			fmt.Printf("防抖延迟: %dms\n", debounceMs)
			fmt.Println("\n按 Ctrl+C 停止...")
			fmt.Println()

			w, err := watcher.New(
				projectPath,
				DbPath,
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

func runInitialAnalysis(projectPath, dbPath string) (nodeCount, edgeCount int64, err error) {
	pkgs, err := analyzer.LoadPackages(projectPath)
	if err != nil {
		return 0, 0, fmt.Errorf("加载包失败: %w", err)
	}

	pkgs = analyzer.FilterMainPackages(pkgs)
	if len(pkgs) == 0 {
		return 0, 0, fmt.Errorf("未找到有效的 Go 包")
	}

	prog, _ := analyzer.BuildSSA(pkgs)

	cg, err := analyzer.BuildCallGraph(prog)
	if err != nil {
		return 0, 0, fmt.Errorf("构建调用图失败: %w", err)
	}

	db, err := storage.Open(dbPath)
	if err != nil {
		return 0, 0, fmt.Errorf("打开数据库失败: %w", err)
	}
	defer db.Close()

	if err := db.Clear(); err != nil {
		return 0, 0, fmt.Errorf("清空数据库失败: %w", err)
	}

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

	interfaceAnalyzer := analyzer.NewInterfaceAnalyzer(pkgs, projectPath)
	_, _, _, _ = interfaceAnalyzer.BuildInterfaceGraph(
		db.InsertNode,
		db.InsertEdge,
	)

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
			db, err := storage.Open(DbPath)
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
