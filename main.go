package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zheng/crag/cmd"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "crag",
		Short: "Code RAG - Go代码调用图分析工具",
		Long: `crag 是一个 Go 代码静态分析工具，用于构建函数调用图谱，
帮助追踪代码变更的影响范围，减少 AI 编码时的漏改问题。`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cmd.DbPath, "db", "d", ".crag.db", "数据库文件路径")

	// Add commands
	cmd.RegisterCommands(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
