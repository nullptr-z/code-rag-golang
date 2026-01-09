# crag 性能评估 Skill

这是一个用于评估 crag 项目性能优化质量的标准化测试流程。每次优化后都应该运行这个测试来量化改进效果。

## 前置条件

1. 确保项目已编译：`go build -o crag main.go`
2. 确保有 mock 项目生成器：`tools/mockgen/main.go`
3. 清理旧的测试数据：`rm -rf ./mock-project ./mock-project/.crag.db`

## 测试流程

### 第一步：生成 Mock 项目

**目的**: 创建一个可控的测试项目，模拟真实项目的复杂度

**执行命令**:
```bash
cd tools/mockgen && go run main.go \
  -pkgs 20 \
  -funcs 100 \
  -depth 10 \
  -density 3.0 \
  -o ../../mock-project

cd ../../mock-project
go mod tidy
```

**参数说明**:
- `pkgs`: 包数量（20 = 中型项目）
- `funcs`: 每包函数数（100）
- `depth`: 调用链深度（10 层）
- `density`: 平均调用密度（每函数调用 3 个其他函数）

**验证**:
```bash
# 应该看到 20 个 pkg00-pkg19 目录
ls mock-project/

# 统计总函数数
find mock-project -name "*.go" -not -name "*_test.go" | xargs grep -c "^func " | awk '{s+=$1} END {print s}'
# 应该输出约 2000
```

**预期结果**:
- 总函数数: 2000
- 总包数: 20
- 无编译错误

---

### 第二步：测试分析性能

#### 2.1 全量分析基准测试

**目的**: 建立性能基准，测试首次完整分析的速度

**执行命令**:
```bash
rm -f ./mock-project/.crag.db

time ./crag analyze ./mock-project -o ./mock-project/.crag.db 2>&1 | tee analysis-full.log
```

**记录指标**:
```bash
# 从输出提取关键数据
grep "完成" analysis-full.log
grep "节点" analysis-full.log
grep "边" analysis-full.log

# 时间（从 time 命令输出）
# real: 实际耗时
# user: CPU 用户态时间
# sys:  CPU 内核态时间
```

**预期结果示例**:
```
完成! 已存储 2000 个函数节点
数据库总计: 2000 节点, 2533 边

real    0m4.281s
user    0m0.050s
sys     0m1.690s
```

**评估标准**:
- ✅ 优秀: < 5 秒
- ⚠️ 可接受: 5-10 秒
- ❌ 需优化: > 10 秒

#### 2.2 增量分析性能测试

**目的**: 测试增量更新的效率，这是日常开发最常用的场景

**执行命令**:
```bash
# 初始化 git 仓库（增量分析需要）
cd mock-project
git init
git add .
git commit -m "initial commit"

# 修改一个包
echo "// test $(date)" >> pkg05/code.go

# 增量分析
cd ..
time ./crag analyze ./mock-project -i -o ./mock-project/.crag.db 2>&1 | tee analysis-incremental.log
```

**记录指标**:
```bash
grep "检测到" analysis-incremental.log
grep "删除" analysis-incremental.log
grep "插入" analysis-incremental.log
grep "完成" analysis-incremental.log
```

**预期结果示例**:
```
检测 git 变更...
检测到 1 个变更文件，涉及 1 个包
转换为完整包路径: [github.com/example/mockproject/pkg05]
增量模式：删除 1 个变更包的旧数据...
已删除 100 个旧节点
增量模式：仅插入变更包的节点
完成! 已存储 100 个函数节点
数据库总计: 2000 节点, 2309 边

real    0m0.203s
```

**计算性能提升**:
```python
speedup = full_time / incremental_time
efficiency = (1 - incremental_time / full_time) * 100

# 示例: 4.281 / 0.203 = 21.1 倍
# 效率: (1 - 0.203/4.281) * 100 = 95.3%
```

**评估标准**:
- ✅ 优秀: > 15 倍加速
- ⚠️ 可接受: 10-15 倍加速
- ❌ 需优化: < 10 倍加速

---

### 第三步：Token 消耗对比测试

#### 3.1 准备测试函数

**选择测试函数**:
```bash
# 找一个有调用关系的函数
cat << 'EOF' | ./crag mcp -d ./mock-project/.crag.db 2>/dev/null | tail -1 | jq -r '.result.content[0].text' | head -5
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search","arguments":{"pattern":"Func0001","limit":5}}}
EOF

# 从输出中选择一个完整的函数名，例如：
# github.com/example/mockproject/pkg00.Func0001
```

#### 3.2 测试 crag MCP 模式

**执行查询**:
```bash
TEST_FUNC="pkg00.Func0001"

cat << EOF | ./crag mcp -d ./mock-project/.crag.db 2>/dev/null | tail -1 | jq -r '.result.content[0].text' > crag-output.txt
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"impact","arguments":{"function":"$TEST_FUNC","limit":10}}}
EOF
```

**统计 Token 消耗**:
```bash
# 字符数
CHARS=$(cat crag-output.txt | wc -c)

# 单词数（英文 Token 近似）
WORDS=$(cat crag-output.txt | wc -w)

# 估算 Token（考虑中文）
# 英文: 1 word ≈ 1 token
# 中文: 1 char ≈ 1.5-2 tokens
# 混合: word_count * 1.3
TOKENS=$((WORDS * 13 / 10))

echo "crag MCP 模式:"
echo "  字符数: $CHARS"
echo "  单词数: $WORDS"
echo "  估算 Token: ~$TOKENS"
```

**显示返回内容**:
```bash
cat crag-output.txt
```

#### 3.3 模拟传统方式

**估算传统方式的 Token 消耗**:

传统方式需要多次往返：

1. **Grep 搜索函数定义** (~500 tokens)
   - 返回所有匹配（包括注释、文档、测试文件）
   - 需要人工或 AI 过滤

2. **Read 确认文件内容** (~2000 tokens)
   - 读取完整文件内容
   - 包含无关函数

3. **Grep 搜索调用者** (~800 tokens)
   - 搜索函数名出现的所有地方
   - 包含字符串、注释中的误报

4. **Read 调用者文件（多次）** (~6000 tokens)
   - 假设有 3 个调用者
   - 每次读取完整文件: 3 × ~2000

5. **Grep 搜索被调用函数** (~500 tokens)

6. **Read 被调用函数文件** (~2000 tokens)

**总计: ~11800 tokens**

#### 3.4 计算节省效果

```bash
CRAG_TOKENS=150  # 从上面的测试获得
TRADITIONAL_TOKENS=11800

SAVED=$((TRADITIONAL_TOKENS - CRAG_TOKENS))
PERCENTAGE=$((SAVED * 100 / TRADITIONAL_TOKENS))

echo "========================"
echo "Token 消耗对比"
echo "========================"
echo "crag 模式:    ~$CRAG_TOKENS tokens"
echo "传统模式:     ~$TRADITIONAL_TOKENS tokens"
echo "节省:         ~$SAVED tokens ($PERCENTAGE%)"
```

**评估标准**:
- ✅ 优秀: 节省 > 95%
- ⚠️ 可接受: 节省 85-95%
- ❌ 需优化: 节省 < 85%

---

### 第四步：MCP 工具分页效果测试

**目的**: 验证结果限制功能是否有效控制 Token 消耗

#### 4.1 不同 limit 参数的 Token 对比

```bash
# 测试不同的 limit 值
for LIMIT in 10 20 50 100 unlimited; do
    if [ "$LIMIT" = "unlimited" ]; then
        ARGS='{"function":"pkg00.Func0001"}'
    else
        ARGS="{\"function\":\"pkg00.Func0001\",\"limit\":$LIMIT}"
    fi

    RESULT=$(cat << EOF | ./crag mcp -d ./mock-project/.crag.db 2>/dev/null | tail -1 | jq -r '.result.content[0].text'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"impact","arguments":$ARGS}}
EOF
)

    WORDS=$(echo "$RESULT" | wc -w)
    TOKENS=$((WORDS * 13 / 10))

    echo "limit=$LIMIT: ~$TOKENS tokens"
done
```

**预期结果**:
```
limit=10: ~120 tokens
limit=20: ~180 tokens
limit=50: ~350 tokens
limit=100: ~650 tokens
unlimited: ~1500 tokens
```

**评估**:
- ✅ Token 随 limit 线性增长
- ✅ 默认 limit=50 在合理范围内
- ✅ 提供总数提示，用户可按需查询更多

---

### 第五步：多场景压力测试

#### 5.1 测试不同项目规模

**小型项目** (500 函数):
```bash
cd tools/mockgen && go run main.go -pkgs 5 -funcs 100 -o ../../mock-project-small
```

**大型项目** (5000 函数):
```bash
cd tools/mockgen && go run main.go -pkgs 50 -funcs 100 -o ../../mock-project-large
```

**记录分析时间**:
```bash
time ./crag analyze ./mock-project-small -o ./mock-project-small/.crag.db
time ./crag analyze ./mock-project-large -o ./mock-project-large/.crag.db
```

#### 5.2 测试不同变更规模

```bash
cd mock-project
git init && git add . && git commit -m "init"

# 场景 1: 修改 1 个包 (5%)
echo "// test" >> pkg05/code.go
time ../crag analyze . -i -o .crag.db

# 场景 2: 修改 5 个包 (25%)
for i in {0..4}; do echo "// test" >> pkg0$i/code.go; done
time ../crag analyze . -i -o .crag.db

# 场景 3: 修改 10 个包 (50%)
for i in {0..9}; do
    if [ $i -lt 10 ]; then
        echo "// test" >> pkg0$i/code.go
    fi
done
time ../crag analyze . -i -o .crag.db
```

---

## 性能报告模板

完成所有测试后，使用以下模板生成报告：

```markdown
# crag 性能评估报告

**测试日期**: $(date +%Y-%m-%d)
**版本**: $(git rev-parse --short HEAD)
**测试环境**: $(uname -s) $(uname -m)

## 1. 分析性能

### Mock 项目 (2000 函数)

| 场景 | 耗时 | 节点数 | 边数 |
|------|------|--------|------|
| 全量分析 | 4.281s | 2000 | 2533 |
| 增量分析 (5%) | 0.203s | 2000 | 2309 |
| **性能提升** | **21.1x** | - | - |

### 不同规模项目

| 规模 | 函数数 | 全量分析 | 增量分析 (5%) | 提升 |
|------|--------|----------|---------------|------|
| 小型 | 500 | 1.2s | 0.08s | 15x |
| 中型 | 2000 | 4.3s | 0.20s | 21x |
| 大型 | 5000 | 12.5s | 0.55s | 23x |

## 2. Token 消耗

### 单次查询对比

| 方式 | Token 消耗 | 节省率 |
|------|-----------|--------|
| crag (limit=10) | ~150 | 98.7% |
| 传统方式 | ~11800 | - |

### 不同 limit 参数

| limit | Token | 适用场景 |
|-------|-------|----------|
| 10 | ~120 | 快速预览 |
| 20 | ~180 | 一般查询 |
| 50 | ~350 | 详细分析 |
| 100 | ~650 | 完整调查 |

## 3. 评分

- **功能完整性**: X/10
- **Token 节省效果**: X/10
- **性能**: X/10
- **易用性**: X/10

**总分**: X/10

## 4. 优化建议

[根据测试结果提出改进建议]
```

---

## 自动化测试脚本

可以将整个流程自动化：

```bash
#!/bin/bash
# tools/run-performance-test.sh

set -e

echo "=== crag 性能评估测试 ==="
echo ""

# 清理
echo "[1/5] 清理旧数据..."
rm -rf ./mock-project ./mock-project-small ./mock-project-large

# 生成项目
echo "[2/5] 生成 mock 项目..."
cd tools/mockgen
go run main.go -pkgs 20 -funcs 100 -depth 10 -density 3.0 -o ../../mock-project
cd ../../mock-project && go mod tidy && cd ..

# 全量分析
echo "[3/5] 全量分析测试..."
time ./crag analyze ./mock-project -o ./mock-project/.crag.db 2>&1 | tee analysis-full.log

# 增量分析
echo "[4/5] 增量分析测试..."
cd mock-project && git init && git add . && git commit -m "init" > /dev/null 2>&1
echo "// test $(date)" >> pkg05/code.go
cd ..
time ./crag analyze ./mock-project -i -o ./mock-project/.crag.db 2>&1 | tee analysis-incremental.log

# Token 测试
echo "[5/5] Token 消耗测试..."
bash tools/token-test.sh

echo ""
echo "=== 测试完成 ==="
echo "查看详细日志: analysis-full.log, analysis-incremental.log"
```

---

## 关键指标总结

每次测试后，重点关注以下指标：

### 性能指标
- [ ] 全量分析耗时 < 5s (2000 函数)
- [ ] 增量分析提速 > 15x
- [ ] 查询响应时间 < 100ms

### Token 效率
- [ ] 单次查询节省 > 95%
- [ ] limit=50 时 < 400 tokens
- [ ] 提供准确的总数提示

### 准确性
- [ ] 节点数正确
- [ ] 边数正确（考虑增量更新）
- [ ] 无循环依赖警告

### 可用性
- [ ] 命令执行无错误
- [ ] 输出格式正确
- [ ] MCP 协议兼容

---

## 使用建议

1. **每次重大优化后运行此测试**
2. **保存历史测试结果进行对比**
3. **关注退化（performance regression）**
4. **根据实际项目调整 mock 项目规模**

---

最后更新: 2026-01-10
