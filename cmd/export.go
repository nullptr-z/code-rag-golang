package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zheng/crag/internal/analyzer"
	"github.com/zheng/crag/internal/export"
	"github.com/zheng/crag/internal/storage"
)

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
			db, err := storage.Open(DbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			exporter := export.NewExporter(db)
			opts := export.DefaultExportOptions()
			opts.IncludeMermaid = !noMermaid

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

			if incremental {
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

			return exporter.Export(w, opts)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "输出文件路径 (默认输出到 stdout)")
	cmd.Flags().BoolVarP(&incremental, "incremental", "i", false, "增量导出 (只输出 git 变更部分)")
	cmd.Flags().StringVar(&gitBase, "base", "HEAD", "git 比较基准")
	cmd.Flags().BoolVar(&noMermaid, "no-mermaid", false, "不生成 Mermaid 图表")

	return cmd
}
