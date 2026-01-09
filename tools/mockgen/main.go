package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// Config represents the mock project configuration
type Config struct {
	OutputDir      string
	NumPackages    int
	NumFuncsPerPkg int
	MaxDepth       int
	CallDensity    float64 // 每个函数平均调用几个其他函数
}

// FuncInfo represents a function in the mock project
type FuncInfo struct {
	Package  string
	Name     string
	FullName string
	Depth    int
	PkgIdx   int
}

// CallInfo represents a function call
type CallInfo struct {
	Package  string
	FuncName string
}

func main() {
	cfg := Config{}
	flag.StringVar(&cfg.OutputDir, "o", "./mock-project", "输出目录")
	flag.IntVar(&cfg.NumPackages, "pkgs", 20, "包数量")
	flag.IntVar(&cfg.NumFuncsPerPkg, "funcs", 100, "每个包的函数数量")
	flag.IntVar(&cfg.MaxDepth, "depth", 10, "最大调用深度")
	flag.Float64Var(&cfg.CallDensity, "density", 3.0, "平均每个函数调用几个其他函数")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	fmt.Printf("正在生成 mock 项目...\n")
	fmt.Printf("  包数量: %d\n", cfg.NumPackages)
	fmt.Printf("  每包函数数: %d\n", cfg.NumFuncsPerPkg)
	fmt.Printf("  总函数数: %d\n", cfg.NumPackages*cfg.NumFuncsPerPkg)
	fmt.Printf("  最大深度: %d\n", cfg.MaxDepth)
	fmt.Printf("  调用密度: %.1f\n", cfg.CallDensity)

	if err := generateProject(&cfg); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ 项目生成完成: %s\n", cfg.OutputDir)
	fmt.Printf("\n下一步:\n")
	fmt.Printf("  cd %s\n", cfg.OutputDir)
	fmt.Printf("  go mod tidy\n")
	fmt.Printf("  crag analyze . -o .crag.db\n")
}

func generateProject(cfg *Config) error {
	// 创建输出目录
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return err
	}

	// 生成 go.mod
	if err := generateGoMod(cfg); err != nil {
		return err
	}

	// 生成所有函数的元信息
	allFuncs := generateFuncRegistry(cfg)

	// 按深度层次组织函数
	funcsByDepth := organizeFuncsByDepth(allFuncs, cfg.MaxDepth)

	// 生成包和代码
	for pkgIdx := 0; pkgIdx < cfg.NumPackages; pkgIdx++ {
		pkgName := fmt.Sprintf("pkg%02d", pkgIdx)
		if err := generatePackage(cfg, pkgName, pkgIdx, funcsByDepth, allFuncs); err != nil {
			return err
		}
		fmt.Printf("  ✓ 生成包 %s (%d/%d)\n", pkgName, pkgIdx+1, cfg.NumPackages)
	}

	return nil
}

func generateGoMod(cfg *Config) error {
	content := `module github.com/example/mockproject

go 1.21
`
	return os.WriteFile(filepath.Join(cfg.OutputDir, "go.mod"), []byte(content), 0644)
}

func generateFuncRegistry(cfg *Config) []*FuncInfo {
	var funcs []*FuncInfo
	for pkgIdx := 0; pkgIdx < cfg.NumPackages; pkgIdx++ {
		pkgName := fmt.Sprintf("pkg%02d", pkgIdx)
		for funcIdx := 0; funcIdx < cfg.NumFuncsPerPkg; funcIdx++ {
			funcName := fmt.Sprintf("Func%04d", funcIdx)
			funcs = append(funcs, &FuncInfo{
				Package:  pkgName,
				Name:     funcName,
				FullName: fmt.Sprintf("%s.%s", pkgName, funcName),
				PkgIdx:   pkgIdx,
			})
		}
	}
	return funcs
}

func organizeFuncsByDepth(allFuncs []*FuncInfo, maxDepth int) [][]*FuncInfo {
	funcsByDepth := make([][]*FuncInfo, maxDepth+1)

	// 均匀分配函数到各个深度层
	for i, fn := range allFuncs {
		depth := i % (maxDepth + 1)
		fn.Depth = depth
		funcsByDepth[depth] = append(funcsByDepth[depth], fn)
	}

	return funcsByDepth
}

func generatePackage(cfg *Config, pkgName string, pkgIdx int, funcsByDepth [][]*FuncInfo, allFuncs []*FuncInfo) error {
	pkgDir := filepath.Join(cfg.OutputDir, pkgName)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return err
	}

	// 计算这个包中的函数范围
	startIdx := pkgIdx * cfg.NumFuncsPerPkg
	endIdx := startIdx + cfg.NumFuncsPerPkg
	pkgFuncs := allFuncs[startIdx:endIdx]

	// 收集需要导入的包
	imports := make(map[string]bool)

	// 生成函数调用关系
	var content string
	content += fmt.Sprintf("package %s\n\n", pkgName)

	// 预先计算所有函数的调用关系
	callMap := make(map[string][]CallInfo)
	for _, fn := range pkgFuncs {
		calls := generateCalls(fn, funcsByDepth, cfg, allFuncs, pkgIdx)
		callMap[fn.Name] = calls
		// 收集导入
		for _, call := range calls {
			if call.Package != pkgName {
				imports[call.Package] = true
			}
		}
	}

	// 写入导入
	if len(imports) > 0 {
		content += "import (\n"
		for imp := range imports {
			content += fmt.Sprintf("\t\"github.com/example/mockproject/%s\"\n", imp)
		}
		content += ")\n\n"
	}

	// 生成每个函数
	for _, fn := range pkgFuncs {
		content += generateFunction(fn, callMap[fn.Name], pkgName)
	}

	return os.WriteFile(filepath.Join(pkgDir, "code.go"), []byte(content), 0644)
}

func generateCalls(fn *FuncInfo, funcsByDepth [][]*FuncInfo, cfg *Config, allFuncs []*FuncInfo, currentPkgIdx int) []CallInfo {
	// 叶子节点（最大深度）不调用其他函数
	if fn.Depth >= len(funcsByDepth)-1 {
		return nil
	}

	// 决定调用多少个函数（泊松分布近似）
	numCalls := rand.Intn(int(cfg.CallDensity*2)) + 1
	if numCalls > int(cfg.CallDensity*1.5) {
		numCalls = int(cfg.CallDensity)
	}

	var calls []CallInfo
	seen := make(map[string]bool)

	// 只调用更深层次的函数，避免循环依赖
	// 优先调用下一层深度的函数（80%概率）
	nextDepth := fn.Depth + 1
	if nextDepth < len(funcsByDepth) && len(funcsByDepth[nextDepth]) > 0 {
		for i := 0; i < numCalls; i++ {
			var target *FuncInfo
			if rand.Float64() < 0.8 && len(funcsByDepth[nextDepth]) > 0 {
				// 调用下一层
				target = funcsByDepth[nextDepth][rand.Intn(len(funcsByDepth[nextDepth]))]
			} else {
				// 随机调用任意深度更深的函数
				deeperFuncs := []*FuncInfo{}
				for d := nextDepth; d < len(funcsByDepth); d++ {
					deeperFuncs = append(deeperFuncs, funcsByDepth[d]...)
				}
				if len(deeperFuncs) > 0 {
					target = deeperFuncs[rand.Intn(len(deeperFuncs))]
				}
			}

			// 避免调用自己，避免重复调用
			// 避免调用自己包的函数（如果跨包），或者只调用更高编号的包
			if target != nil && target.FullName != fn.FullName && !seen[target.FullName] {
				// 同包调用或者调用更高编号的包（避免循环导入）
				if target.PkgIdx == currentPkgIdx || target.PkgIdx > currentPkgIdx {
					calls = append(calls, CallInfo{
						Package:  target.Package,
						FuncName: target.Name,
					})
					seen[target.FullName] = true
				}
			}
		}
	}

	return calls
}

func generateFunction(fn *FuncInfo, calls []CallInfo, currentPkg string) string {
	var content string

	// 添加文档注释
	content += fmt.Sprintf("// %s is a mock function at depth %d\n", fn.Name, fn.Depth)
	content += fmt.Sprintf("// This function represents a node in the call graph for testing purposes.\n")

	// 函数签名
	content += fmt.Sprintf("func %s(input int) int {\n", fn.Name)

	// 函数体
	content += "\tresult := input\n"

	// 生成调用
	for i, call := range calls {
		var callExpr string
		if call.Package == currentPkg {
			// 同包调用
			callExpr = call.FuncName
		} else {
			// 跨包调用
			callExpr = fmt.Sprintf("%s.%s", call.Package, call.FuncName)
		}
		content += fmt.Sprintf("\tresult += %s(result + %d)\n", callExpr, i)
	}

	content += "\treturn result\n"
	content += "}\n\n"

	return content
}
