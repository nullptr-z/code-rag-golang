package display

import (
	"fmt"
	"strings"

	"github.com/zheng/crag/internal/storage"
)

// ShortFuncName simplifies a fully qualified function name.
// e.g., "(*github.com/foo/bar/pkg.Type).Method" -> "(*pkg.Type).Method"
// e.g., "github.com/foo/bar/pkg.FuncName" -> "pkg.FuncName"
func ShortFuncName(fullName string) string {
	prefix := ""
	name := fullName
	if strings.HasPrefix(name, "(*") {
		prefix = "(*"
		name = name[2:]
	} else if strings.HasPrefix(name, "(") {
		prefix = "("
		name = name[1:]
	}

	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	return prefix + name
}

// ShortSignature simplifies package paths in a function signature.
// e.g., "func(db *github.com/jinzhu/gorm.DB) error" -> "func(db *gorm.DB) error"
func ShortSignature(sig string) string {
	result := sig
	for {
		start := -1
		for i := 0; i < len(result); i++ {
			if result[i] == '/' {
				start = i
				for j := i - 1; j >= 0; j-- {
					c := result[j]
					if c == ' ' || c == '*' || c == '(' || c == '[' || c == ',' {
						start = j + 1
						break
					}
					if j == 0 {
						start = 0
					}
				}
				break
			}
		}
		if start == -1 {
			break
		}

		lastSlash := -1
		for i := start; i < len(result); i++ {
			if result[i] == '/' {
				lastSlash = i
			}
			if result[i] == ' ' || result[i] == ')' || result[i] == ',' || result[i] == ']' {
				break
			}
		}

		if lastSlash > start {
			result = result[:start] + result[lastSlash+1:]
		} else {
			break
		}
	}
	return result
}

// CalcTreeMaxWidth calculates the maximum function name width and depth for alignment in the call tree.
func CalcTreeMaxWidth(tree []*storage.CallTreeNode, maxWidth *int, currentDepth int, maxDepth *int) {
	if currentDepth > *maxDepth {
		*maxDepth = currentDepth
	}
	for _, node := range tree {
		w := len(ShortFuncName(node.Node.Name))
		if w > *maxWidth {
			*maxWidth = w
		}
		if len(node.Children) > 0 {
			CalcTreeMaxWidth(node.Children, maxWidth, currentDepth+1, maxDepth)
		}
	}
}

// FormatCallTree renders a call tree as a string with ASCII art box-drawing characters.
func FormatCallTree(tree []*storage.CallTreeNode, indent string, maxWidth int, maxDepth int, currentDepth int) string {
	var sb strings.Builder
	for i, node := range tree {
		isLast := i == len(tree)-1
		prefix := "├──"
		if isLast {
			prefix = "└──"
		}

		funcName := ShortFuncName(node.Node.Name)
		loc := fmt.Sprintf("%s:%d", node.Node.File, node.Node.Line)
		padding := maxWidth + (maxDepth-currentDepth)*4
		sb.WriteString(fmt.Sprintf("%s%s %-*s  %s\n", indent, prefix, padding, funcName, loc))

		if len(node.Children) > 0 {
			childIndent := indent + "│   "
			if isLast {
				childIndent = indent + "    "
			}
			sb.WriteString(FormatCallTree(node.Children, childIndent, maxWidth, maxDepth, currentDepth+1))
		}
	}
	return sb.String()
}
