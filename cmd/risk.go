package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zheng/crag/internal/storage"
)

func riskCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "risk [function-name]",
		Short: "分析函数变更风险",
		Long: `分析函数的变更风险等级，基于调用者数量评估。

风险等级说明：
  - critical: 直接调用者 >= 50 或总调用者 >= 200
  - high:     直接调用者 >= 20 或总调用者 >= 100
  - medium:   直接调用者 >= 5 或总调用者 >= 30
  - low:      其他

示例：
  crag risk HandleRequest   # 查看单个函数的风险
  crag risk --top 20        # 显示风险最高的20个函数`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			showTop, _ := cmd.Flags().GetBool("top")

			db, err := storage.Open(DbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			if showTop || len(args) == 0 {
				risks, err := db.GetTopRiskyFunctions(limit)
				if err != nil {
					return fmt.Errorf("查询失败: %w", err)
				}

				if len(risks) == 0 {
					fmt.Println("项目中没有函数")
					return nil
				}

				fmt.Printf("高风险函数排行 (Top %d)\n\n", limit)
				for _, r := range risks {
					riskIcon := getRiskIcon(r.RiskLevel)
					fmt.Printf("%s %-8s  %s\n", riskIcon, r.RiskLevel, shortFuncName(r.Node.Name))
					fmt.Printf("             调用者: %d  %s:%d\n\n", r.DirectCallers, r.Node.File, r.Node.Line)
				}

				fmt.Println("风险等级: 🔴critical(>=50) 🟠high(>=20) 🟡medium(>=5) 🟢low")
				fmt.Println("\n💡 使用 crag risk <函数名> 查看详细分析")
				return nil
			}

			funcName := args[0]

			nodes, err := db.FindNodesByPattern(funcName)
			if err != nil {
				return fmt.Errorf("查询失败: %w", err)
			}

			if len(nodes) == 0 {
				return fmt.Errorf("未找到函数: %s", funcName)
			}

			node := nodes[0]
			risk, err := db.GetRiskScore(node.ID)
			if err != nil {
				return fmt.Errorf("计算风险失败: %w", err)
			}

			riskIcon := getRiskIcon(risk.RiskLevel)
			fmt.Printf("## 变更风险分析: %s\n\n", shortFuncName(risk.Node.Name))
			fmt.Printf("**位置:** %s:%d\n", risk.Node.File, risk.Node.Line)
			if risk.Node.Signature != "" {
				fmt.Printf("**签名:** `%s`\n", shortSignature(risk.Node.Signature))
			}
			fmt.Println()

			fmt.Printf("### 风险等级: %s %s\n\n", riskIcon, risk.RiskLevel)
			fmt.Printf("直接调用者: %d\n", risk.DirectCallers)

			fmt.Println("\n**建议:**")
			switch risk.RiskLevel {
			case "critical":
				fmt.Println("- ⚠️  此函数被大量调用，修改需极其谨慎")
				fmt.Println("- 建议先运行 `crag impact` 查看完整影响范围")
				fmt.Println("- 修改前确保有充分的测试覆盖")
				fmt.Println("- 考虑是否可以添加新函数而非修改现有函数")
			case "high":
				fmt.Println("- ⚠️  此函数调用者较多，修改需谨慎")
				fmt.Println("- 建议运行 `crag upstream` 查看调用者")
				fmt.Println("- 确保修改后同步更新所有调用处")
			case "medium":
				fmt.Println("- 正常风险，注意检查调用处是否需要同步修改")
				fmt.Println("- 可运行 `crag upstream` 查看具体调用者")
			case "low":
				fmt.Println("- 低风险，影响范围较小")
				fmt.Println("- 正常修改即可")
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "显示数量")
	cmd.Flags().Bool("top", false, "显示风险最高的函数列表")

	return cmd
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
