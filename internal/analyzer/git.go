package analyzer

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitChanges represents the result of git diff analysis
type GitChanges struct {
	ChangedFiles   []string // 变更的 .go 文件
	ChangedPackages []string // 变更文件所属的包目录
}

// GetGitChanges returns the list of changed Go files since the last commit
// If base is empty, it compares with HEAD (uncommitted changes)
// If base is "HEAD~1", it compares with the previous commit
func GetGitChanges(projectPath string, base string) (*GitChanges, error) {
	if base == "" {
		base = "HEAD"
	}

	// Get list of changed files
	cmd := exec.Command("git", "diff", "--name-only", base)
	cmd.Dir = projectPath

	output, err := cmd.Output()
	if err != nil {
		// If git diff HEAD fails (e.g., no commits yet), try getting all tracked files
		cmd = exec.Command("git", "ls-files", "--modified", "--others", "--exclude-standard")
		cmd.Dir = projectPath
		output, err = cmd.Output()
		if err != nil {
			return nil, err
		}
	}

	changes := &GitChanges{
		ChangedFiles:    make([]string, 0),
		ChangedPackages: make([]string, 0),
	}

	pkgSet := make(map[string]bool)

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		file := strings.TrimSpace(scanner.Text())
		if file == "" {
			continue
		}

		// Only include .go files
		if !strings.HasSuffix(file, ".go") {
			continue
		}

		// Skip test files for now
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		changes.ChangedFiles = append(changes.ChangedFiles, file)

		// Extract package directory
		pkgDir := filepath.Dir(file)
		if pkgDir == "." {
			pkgDir = "./"
		} else {
			pkgDir = "./" + pkgDir
		}

		if !pkgSet[pkgDir] {
			pkgSet[pkgDir] = true
			changes.ChangedPackages = append(changes.ChangedPackages, pkgDir)
		}
	}

	return changes, scanner.Err()
}

// HasChanges returns true if there are any Go file changes
func (g *GitChanges) HasChanges() bool {
	return len(g.ChangedFiles) > 0
}

// String returns a summary string of the changes
func (g *GitChanges) String() string {
	return fmt.Sprintf("%d files changed in %d packages", len(g.ChangedFiles), len(g.ChangedPackages))
}

// GetChangedPackagePatterns returns package patterns for go/packages.Load
func (g *GitChanges) GetChangedPackagePatterns() []string {
	if len(g.ChangedPackages) == 0 {
		return []string{"./..."}
	}
	return g.ChangedPackages
}

// GetRemoteTrackingBranch 获取当前分支对应的远程跟踪分支
// 返回格式如 "origin/main" 或 "origin/feature-branch"
func GetRemoteTrackingBranch(projectPath string) (string, error) {
	// 使用 git rev-parse 获取上游分支
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = projectPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("无法获取远程跟踪分支: %w", err)
	}

	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "", fmt.Errorf("当前分支没有设置远程跟踪分支")
	}

	return branch, nil
}

