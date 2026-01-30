package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/zheng/crag/internal/storage"
)

func outputJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// shortFuncName simplifies a fully qualified function name
// e.g., "(*github.com/foo/bar/pkg.Type).Method" -> "(*pkg.Type).Method"
func shortFuncName(fullName string) string {
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

// shortFilePath returns the file path as-is (already relative to project root)
func shortFilePath(fullPath string) string {
	return fullPath
}

// shortSignature simplifies package paths in a function signature
// e.g., "func(db *github.com/jinzhu/gorm.DB) error" -> "func(db *gorm.DB) error"
func shortSignature(sig string) string {
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

// calcTreeMaxWidth calculates the maximum function name width and depth for alignment in the call tree
func calcTreeMaxWidth(tree []*storage.CallTreeNode, maxWidth *int, currentDepth int, maxDepth *int) {
	if currentDepth > *maxDepth {
		*maxDepth = currentDepth
	}
	for _, node := range tree {
		w := len(shortFuncName(node.Node.Name))
		if w > *maxWidth {
			*maxWidth = w
		}
		if len(node.Children) > 0 {
			calcTreeMaxWidth(node.Children, maxWidth, currentDepth+1, maxDepth)
		}
	}
}

// printCallTree prints the call tree with proper indentation
func printCallTree(tree []*storage.CallTreeNode, indent string, isUpstream bool, maxWidth int, maxDepth int, currentDepth int) {
	for i, node := range tree {
		isLast := i == len(tree)-1
		prefix := "├──"
		if isLast {
			prefix = "└──"
		}

		funcName := shortFuncName(node.Node.Name)
		loc := fmt.Sprintf("%s:%d", shortFilePath(node.Node.File), node.Node.Line)
		padding := maxWidth + (maxDepth-currentDepth)*4
		fmt.Printf("%s%s %-*s  %s\n", indent, prefix, padding, funcName, loc)

		if len(node.Children) > 0 {
			childIndent := indent + "│   "
			if isLast {
				childIndent = indent + "    "
			}
			printCallTree(node.Children, childIndent, isUpstream, maxWidth, maxDepth, currentDepth+1)
		}
	}
}
