# crag 工作日志

## 2026-08-13：MCP 自动维护索引 + 非 Go 项目防护

### 起因

用户反馈 `crag` 这个 MCP 几乎没在用起来，CC 调用经常失败转 grep。排查发现两层问题：

1. 全局几十个项目的 `.crag.db` 大部分是空模板（0 节点），从没真正跑过 `crag analyze`，一查就是"未找到函数"。
2. `crag` 只支持 Go，但被挂在了 Node.js 项目上（如 `audio-chatroom-api`），天生查不出结果——但仍然会在这些项目目录下生成一个没用的空 `.crag.db` 文件。

用户要求：
- 非 Go 项目不该创建 `.crag.db`
- 建索引 / 保鲜索引这两件事应该由 MCP 自动做，不该靠人或 AI 记得手动跑命令

### 改动（未提交到 git，工作区改动）

- `internal/analyzer/loader.go`：新增 `IsGoProject(projectPath)`，检查 `go.mod` 是否存在（含一层子目录），供调用方在动数据库之前先判断。
- `cmd/server.go`：
  - `mcpCmd`：启动时先用 `IsGoProject` 校验，不是 Go 项目直接报错退出，不落地任何 `.crag.db`。是 Go 项目则 `ensureIndexed`（索引为空/缺失时自动做一次全量分析），随后默认开启后台 `watcher`（新增 `--watch`，默认 true，`--debounce` 可调），运行期间自动监控文件变更增量重建索引。
  - `analyze` 命令同样补上 `IsGoProject` 校验，行为一致。
  - `runInitialAnalysis` 补上了原来遗漏的变量/常量分析这一步（跟 `analyze` 命令保持一致）。
- `internal/mcp/server.go`：`tools/list` 里的索引维护提示从"改完必须手动 `crag analyze -i -r`"改成"索引会自动更新，无需手动跑"，跟新行为对齐。
- `internal/watcher/watcher.go`：**修了一个一直存在的真 bug**——`addDirs` 用 `filepath.Walk` 递归加监控目录时，靠"目录名前缀是 `.` 就跳过"来排除 `.git` 等隐藏目录，但 `filepath.Walk` 对根路径的第一次回调，`info.Name()` 在传入相对路径 `"."` 时也返回 `"."`，正好命中这条跳过规则——导致 `crag mcp .`（最常见的默认调用方式）从一开始 watch 就完全没在监控任何目录，`--watch` 形同摆设。这也是"索引一直是陈旧的，CC 查不到东西才转 grep"的一部分根因。修法：根路径本身跳过这条隐藏名判断。

### 验证

- `go build ./...`、`go vet ./...` 全绿。
- 用真实编译出的二进制端到端测试过：
  - 非 Go 项目跑 `crag mcp .` → 直接报错退出，不生成 `.crag.db`。
  - Go 项目、`.crag.db` 为空 → 启动自动全量分析，紧接着查询立即可用。
  - Go 项目运行期间编辑代码 → 后台 watcher 在 debounce 后自动增量重建索引，无需任何手动命令，查询能拿到刚加的函数。
- 注册在 `~/.claude.json` 的 MCP 二进制（`code-rag-golang/crag`）已重新编译，binary mtime 晚于所有改动过的源文件，改动已生效。**MCP server 是随会话启动的常驻进程，如果当前会话已经拉起过旧版 `crag mcp`，需要开新会话/重启 Claude Code 才会真正吃到新二进制。**

### 本轮实施状态

- 已将函数节点枚举从调用图解耦：`internal/graph/builder.go` 从 AST/Types 写入 `FuncDecl`，调用图仅补调用边，并允许 nil 调用图。
- 默认调用图改为 `static.CallGraph`，降低 VTA 的资源压力。
- `cmd/server.go` 初始分析与 `internal/watcher/watcher.go` 重建改用临时数据库，完成后通过 `storage.Publish` 事务发布，分析失败不会先清空正式库。
- `ensureIndexed` 增加函数节点计数判断，残缺的“总节点非零但函数极少”索引不再直接复用。
- 已验证：`gofmt`、`go test ./...`、`go vet ./...`、`go build ./...` 全部通过。
- 尚未完成：增量模式已有函数 ID 预加载、完整函数覆盖率元数据、回归测试、live-activity 端到端验证。
