package graph

// EdgeKind represents the type of relationship between nodes
type EdgeKind string

const (
	EdgeKindCalls      EdgeKind = "calls"
	EdgeKindImplements EdgeKind = "implements"
	EdgeKindReferences EdgeKind = "references"
)

// Edge represents a relationship between two nodes
type Edge struct {
	ID           int64    `json:"id"`
	FromID       int64    `json:"from_id"`
	ToID         int64    `json:"to_id"`
	Kind         EdgeKind `json:"kind"`
	CallSiteFile string   `json:"call_site_file"` // 调用发生的文件
	CallSiteLine int      `json:"call_site_line"` // 调用发生的行号
}

