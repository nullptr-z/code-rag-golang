package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zheng/crag/internal/display"
	"github.com/zheng/crag/internal/graph"
	"github.com/zheng/crag/internal/storage"
)

func implementsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "implements <interface-or-type>",
		Short: "查询接口实现关系",
		Long: `查询接口的实现类型，或类型实现的接口。

示例：
  crag implements Reader       # 查询谁实现了 Reader 接口
  crag implements MyStruct     # 查询 MyStruct 实现了哪些接口
  crag implements --list       # 列出所有接口`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			listAll, _ := cmd.Flags().GetBool("list")

			db, err := storage.Open(DbPath)
			if err != nil {
				return fmt.Errorf("打开数据库失败: %w", err)
			}
			defer db.Close()

			if listAll {
				interfaces, err := db.GetAllInterfaces()
				if err != nil {
					return fmt.Errorf("查询接口失败: %w", err)
				}

				if len(interfaces) == 0 {
					fmt.Println("项目中没有接口定义")
					fmt.Println("\n💡 提示：请先运行 analyze 命令分析项目：")
					fmt.Println("   crag analyze .")
					return nil
				}

				fmt.Printf("项目接口列表 (共 %d 个)\n\n", len(interfaces))
				for _, iface := range interfaces {
					methods := display.ShortSignature(iface.Signature)
					if methods == "" {
						methods = "(空接口)"
					}
					fmt.Printf("  %s\n", display.ShortFuncName(iface.Name))
					fmt.Printf("    方法: %s\n", methods)
					fmt.Printf("    位置: %s:%d\n\n", iface.File, iface.Line)
				}
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("请提供接口或类型名称，或使用 --list 列出所有接口")
			}

			name := args[0]

			interfaces, err := db.FindInterfacesByPattern(name)
			if err != nil {
				return fmt.Errorf("查询失败: %w", err)
			}

			if len(interfaces) > 0 {
				iface := interfaces[0]
				if len(interfaces) > 1 {
					fmt.Printf("找到 %d 个匹配的接口，显示第一个:\n\n", len(interfaces))
				}

				fmt.Printf("接口: %s\n", display.ShortFuncName(iface.Name))
				fmt.Printf("位置: %s:%d\n", iface.File, iface.Line)
				if iface.Signature != "" {
					fmt.Printf("方法: %s\n", display.ShortSignature(iface.Signature))
				}
				fmt.Println()

				impls, err := db.GetImplementations(iface.ID)
				if err != nil {
					return fmt.Errorf("查询实现失败: %w", err)
				}

				if len(impls) == 0 {
					fmt.Println("没有找到实现此接口的类型")
				} else {
					fmt.Printf("实现类型 (共 %d 个):\n\n", len(impls))
					for _, impl := range impls {
						fmt.Printf("  %s\n", display.ShortFuncName(impl.Name))
						fmt.Printf("    %s:%d\n", impl.File, impl.Line)
					}
				}
				return nil
			}

			types, err := db.FindNodesByPattern(name)
			if err != nil {
				return fmt.Errorf("查询失败: %w", err)
			}

			var structTypes []*graph.Node
			for _, t := range types {
				if t.Kind == graph.NodeKindStruct {
					structTypes = append(structTypes, t)
				}
			}

			if len(structTypes) > 0 {
				typ := structTypes[0]
				if len(structTypes) > 1 {
					fmt.Printf("找到 %d 个匹配的类型，显示第一个:\n\n", len(structTypes))
				}

				fmt.Printf("类型: %s\n", display.ShortFuncName(typ.Name))
				fmt.Printf("位置: %s:%d\n\n", typ.File, typ.Line)

				implInterfaces, err := db.GetImplementedInterfaces(typ.ID)
				if err != nil {
					return fmt.Errorf("查询接口失败: %w", err)
				}

				if len(implInterfaces) == 0 {
					fmt.Println("此类型没有实现任何接口")
				} else {
					fmt.Printf("实现的接口 (共 %d 个):\n\n", len(implInterfaces))
					for _, iface := range implInterfaces {
						methods := display.ShortSignature(iface.Signature)
						if methods == "" {
							methods = "(空接口)"
						}
						fmt.Printf("  %s\n", display.ShortFuncName(iface.Name))
						fmt.Printf("    方法: %s\n", methods)
						fmt.Printf("    位置: %s:%d\n\n", iface.File, iface.Line)
					}
				}
				return nil
			}

			fmt.Printf("未找到名为 '%s' 的接口或类型\n", name)
			fmt.Println("\n💡 提示：请先运行 analyze 命令分析项目：")
			fmt.Println("   crag analyze .")
			return nil
		},
	}

	cmd.Flags().Bool("list", false, "列出所有接口")

	return cmd
}
