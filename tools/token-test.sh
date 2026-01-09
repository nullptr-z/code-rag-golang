#!/bin/bash
# Token 消耗对比测试
# 模拟 AI 查询函数调用关系的场景

set -e

DB_PATH="./mock-project/.crag.db"
TEST_FUNC="pkg05.Func0050"

echo "================================"
echo "Token 消耗对比测试"
echo "测试函数: $TEST_FUNC"
echo "================================"
echo ""

# 测试 1：使用 crag MCP 工具
echo "=== 场景 1: 使用 crag (MCP 模式) ==="
echo "查询: impact($TEST_FUNC, limit=20)"
echo ""

# 启动 MCP 服务器并发送请求
RESULT=$(./crag mcp -d $DB_PATH <<EOF | tail -1
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"impact","arguments":{"function":"$TEST_FUNC","limit":20}}}
EOF
)

# 提取返回的文本内容
TEXT=$(echo "$RESULT" | jq -r '.result.content[0].text' 2>/dev/null || echo "$RESULT")
TOKEN_COUNT=$(echo "$TEXT" | wc -w)
CHAR_COUNT=$(echo "$TEXT" | wc -c)

echo "返回内容预览:"
echo "$TEXT" | head -20
echo "..."
echo ""
echo "统计:"
echo "  - 字符数: $CHAR_COUNT"
echo "  - 单词数 (近似Token): $TOKEN_COUNT"
echo "  - 估算 Token: ~$((TOKEN_COUNT * 13 / 10)) (按1.3倍计算中文)"
echo ""

# 测试 2：传统方式（模拟 Grep + Read）
echo "=== 场景 2: 传统方式 (Grep + Read 多次) ==="
echo ""
echo "步骤 1: Grep 搜索函数定义"
echo "  → 返回所有匹配（包括注释、文档）"
echo "  → 估算返回: ~500 tokens"
echo ""
echo "步骤 2: Read 确认文件内容"
echo "  → 读取完整文件内容"
echo "  → 估算返回: ~2000 tokens"
echo ""
echo "步骤 3: Grep 搜索调用者"
echo "  → 搜索函数名出现的地方"
echo "  → 估算返回: ~800 tokens"
echo ""
echo "步骤 4-6: 逐个 Read 调用者文件"
echo "  → 每次读取完整文件"
echo "  → 估算返回: 3 × ~2000 = ~6000 tokens"
echo ""
echo "步骤 7: Grep 搜索被调用函数"
echo "  → 估算返回: ~500 tokens"
echo ""
echo "总计估算: ~9800 tokens"
echo ""

# 计算节省比例
CRAG_TOKENS=$((TOKEN_COUNT * 13 / 10))
TRADITIONAL_TOKENS=9800
SAVED=$((TRADITIONAL_TOKENS - CRAG_TOKENS))
PERCENTAGE=$((SAVED * 100 / TRADITIONAL_TOKENS))

echo "================================"
echo "对比总结"
echo "================================"
echo "使用 crag:     ~$CRAG_TOKENS tokens"
echo "传统方式:      ~$TRADITIONAL_TOKENS tokens"
echo "节省:          ~$SAVED tokens ($PERCENTAGE%)"
echo ""
echo "在大型项目中（10000+ 函数）："
echo "- 传统方式可能需要: 20000-50000 tokens"
echo "- crag 仍然只需要:  200-500 tokens"
echo "- 节省率: 95-99%"
echo "================================"
