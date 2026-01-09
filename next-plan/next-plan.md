## Go 项目图谱驱动的 AI 编程辅助落地方案

**0. 核心目标**

利用已构建的函数调用图谱（Call Graph），解决 AI 在大规模 Go 项目中的**“修改不彻底”和“上下文丢失”问题，实现“牵一发而动全身”**的精确代码演进。

---
## 1. 阶段一：上下文剪枝与 Prompt 注入策略

即便有了图谱，也不能将全量图数据丢给 AI。我们需要一套算法来提取“相关子图”。

1.1 影响面扫描 (Impact Radius)当用户提出修改需求时，系统需执行以下操作：
- 起始点定位： 通过语义搜索（Vector Search）或精确匹配定位到需求涉及的核心函数/结构体。
- 拓扑展开： * 向上（Upstream）： 寻找所有调用该函数的 Caller。向下（Downstream）： 寻找该函数依赖的所有子函数及 Interface 实现。
- 深度控制： 通常设定深度为 2-3 层。

1.2 结构化上下文构造
将提取的子图转换为 AI 易读的 XML 或 JSON 结构：
```xml
XML<context>
  <target_function name="UpdateUser">
    <source_code>...</source_code>
    <callers>
      <function name="UserHandler" file="api/user.go" />
    </callers>
    <dependencies>
      <function name="db.Save" file="internal/db/db.go" />
    </dependencies>
  </target_function>
</context>
```

## 2. 阶段二：建立“修改契约”工作流 (Planning)

在 AI 动笔写代码前，先让它生成一个修改计划书。

2.1 任务拆解 (Multi-step Planning)

要求 AI 输出以下格式的计划：

1. 直接修改点： 哪些函数需要直接改逻辑。
2. 连锁反应点： 因为接口变动，哪些上游调用方需要同步更新。
3. 副作用评估： 修改后是否需要更新对应的单元测试（Unit Test）。

2.2 用户审批环节

系统展示该计划。用户确认后，AI 进入“自动驾驶”模式。这一步是缓解“AI 乱改”的关键防线。

---

3. 阶段三：原子化执行与迭代循环

利用 Go 编译速度快的优势，将修改过程变成一个“闭环”。

3.1 循环修改流程

1. 逐个节点修改： AI 按照计划书中的节点顺序执行修改（建议从叶子节点到根节点）。
2. 即时静态检查： 每完成一个节点的修改，自动运行 go check 或 go build。
3. 反馈修正： 如果编译报错（如参数不匹配），将 Error 信息反馈给 AI，结合图谱告诉它：“你改了 A，但 B 的调用还没更新”。

---

4. 阶段四：图谱实时同步 (Self-Healing Graph)

代码改变后，原始的 RAG 图谱会过时。

- 局部增量更新： 监控文件变动，仅对被修改的文件重新运行 go/ast 解析。
- 同步索引： 更新向量数据库和图数据库中的节点属性（尤其是函数签名和文档注释）。

---

5. 建议的工具链组合

角色推荐方案

Orchestration (编排)使用 LangGraph。它非常适合处理这种带循环（修改->失败->再修改）的图工作流。UI 交互开发一个简单的 CLI 工具 或 VS Code Plugin。验证器直接调用原生 go fmt, go vet 和 golangci-lint。

6. 后续具体 Action Item (下一步行动)

1. [代码实现] 编写一个 Graph Navigator 模块：输入函数名，返回其 $N$ 层内所有的邻居节点代码片段。
2. [Prompt 模板] 编写一个 System Prompt，强制 AI 在回答前必须先输出：Affected Nodes: [A, B, C]。
3. [集成测试] 找一个有 Interface 定义的复杂 Go 模块，尝试让 AI 修改 Interface 定义，观察它能否通过图谱推导出所有隐式实现类（Struct）都需要修改。
