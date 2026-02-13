package graph

// NodeKind represents the type of a code element
type NodeKind string

const (
	NodeKindFunc      NodeKind = "func"
	NodeKindStruct    NodeKind = "struct"
	NodeKindInterface NodeKind = "interface"
	NodeKindPackage   NodeKind = "package"
	NodeKindVar       NodeKind = "var"
	NodeKindConst     NodeKind = "const"
)

// Node represents a code element in the call graph
type Node struct {
	ID        int64    `json:"id"`
	Kind      NodeKind `json:"kind"`
	Name      string   `json:"name"`      // 完整限定名 (pkg.FuncName)
	Package   string   `json:"package"`   // 包路径
	File      string   `json:"file"`      // 源文件路径
	Line      int      `json:"line"`      // 起始行号
	Signature string   `json:"signature"` // 函数签名
	Doc       string   `json:"doc"`       // 文档注释
}

