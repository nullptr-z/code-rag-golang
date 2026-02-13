package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/zheng/crag/internal/display"
	"github.com/zheng/crag/internal/storage"
)

func outputJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// shortFilePath returns the file path as-is (already relative to project root)
func shortFilePath(fullPath string) string {
	return fullPath
}

// printCallTree prints the call tree to stdout
func printCallTree(tree []*storage.CallTreeNode, indent string, isUpstream bool, maxWidth int, maxDepth int, currentDepth int) {
	fmt.Print(display.FormatCallTree(tree, indent, maxWidth, maxDepth, currentDepth))
}
