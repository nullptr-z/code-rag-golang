# CRAG - Code RAG for Go

Go 代码调用图分析工具，支持 CLI 和 MCP 两种模式。

## 构建

```bash
go build -o ./crag .
```

## CLI 使用说明

### 多匹配函数的处理

当函数名匹配到多个结果时（如 `AddUserRankScore` 同时存在于多个包），CLI 命令会显示候选列表并等待交互输入。

**在非交互环境中（如 AI 调用），请使用 `--select` 标志直接指定序号，跳过交互提示：**

```bash
# 先不带 --select 运行，查看候选列表
./crag impact "AddUserRankScore"
# 输出:
#   [1] livepkseason.AddUserRankScore
#       models/livedb/livepkseason/seasonuserrank.go:56
#   [2] livemulticonnect.AddUserRankScore
#       models/livedb/livemulticonnect/userrank.go:176

# 然后用 --select 选择目标
./crag impact --select 1 "AddUserRankScore"
```

以下命令支持 `--select`：
- `crag impact --select N <function-name>`
- `crag upstream --select N <function-name>`
- `crag downstream --select N <function-name>`
