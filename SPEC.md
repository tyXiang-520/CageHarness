# SPEC · Coding Agent Harness

> **Spec-Driven, Subagent-Built, Human-Owned.**
>
> 项目：AI4SE 期末项目 — A · Coding Agent Harness
> 技术栈：Go 1.22+
> 深入维度：治理（Governance — 护栏 + 沙箱 + HITL + 审计）
> 版本：V3.0（与实现同步）

---

## 目录

1. [问题陈述](#1-问题陈述)
2. [用户故事](#2-用户故事)
3. [功能规约](#3-功能规约)
4. [非功能性需求](#4-非功能性需求)
5. [系统架构与领域机制设计](#5-系统架构与领域机制设计)
6. [数据模型](#6-数据模型)
7. [凭据与分发设计](#7-凭据与分发设计)
8. [技术选型与理由](#8-技术选型与理由)
9. [验收标准](#9-验收标准)
10. [风险与未决问题](#10-风险与未决问题)

---

## 1. 问题陈述

### 1.1 要解决什么问题

当前，LLM（Large Language Model）在代码生成方面已展现出强大能力。但一个"只会生成代码"的 LLM 并不是一个可靠的工程工具——它无法自主验证输出是否正确、无法在犯错时自我修正、无法区分"安全操作"与"危险操作"，也无法在跨会话协作中持续遵循项目约定。

**核心等式：Agent = LLM + Harness。**

LLM 相当于 CPU，只负责"决定下一步做什么"这一行任务决策；其余都是工程：把一个只会产生下一步设想的 LLM，封装成一台能稳定、可靠工作的系统。一个 harness 至少需要用代码实现：决策封装、动作/工具、上下文与记忆、治理护栏、反馈闭环、配置。

本项目的命题落在这层工程上：当 LLM 能完成大部分"思考"时，工程师的价值落在 harness 这层工程（治理、反馈、上下文、安全、分发）。

### 1.2 目标用户

- **主要受众**：课程评审老师。需要代码可读性高、文档完备、机制可演示、可确定性测试，能直观展示 Harness 各维度的工程实现。
- **次要受众**：开发者本人及兼容其他开发者简易试用。具备基础代码修复、测试生成、代码重构能力，可作为个人本地 coding 辅助工具。
- **评审者视角**：项目可在一台全新机器上，从零运行、安全配置 key、启动 CLI 或 WebUI，完整观察 Agent 从任务输入到执行完成的全部过程。

### 1.3 为什么值得做

市面上已有 LangChain、AutoGen、CrewAI 等 Agent 框架，但它们的高层抽象隐藏了治理、反馈、记忆等关键机制的实现细节。本项目做一个**最小但完整的、机制透明的、可确定性测试的** Coding Agent Harness，核心价值在于：

1. **教学价值**：清晰展示 Agent = LLM + Harness 的完整工程闭环，每个机制都是代码而非提示词。
2. **工程价值**：展示治理护栏、反馈闭环、按需记忆等机制如何让一个 LLM 从"不可靠的文本生成器"变为"可治理的工程系统"。
3. **实验价值**：以治理（Governance）为深入维度，探索 Risk Classification → Policy Engine → Execution Boundary → Execution Control → HITL → Audit Log 的完整可审计治理管线。

---

## 2. 用户故事

### 2.1 US-01：安全护栏拦截危险操作并人工审批

> **作为** 开发者，
> **我希望** 当我（或 Agent）发起一项危险的 coding 操作时，Harness 能自动识别风险等级、解释拦截原因，并在我批准后才执行，
> **以便** 我不会因为 Agent 的误操作而丢失代码或破坏项目状态。

**Non-goals**：不实现生产级 OS 隔离，不保证针对恶意宿主用户的安全。仅提供 workspace boundary、policy、approval 等 Harness 级治理。

**验收标准**：

| AC# | 描述 |
|-----|------|
| US01-AC01 | 当 Agent 产生 `execute_shell` Action 且命令匹配 `git reset --hard` 规则时，Governance 将其分类为 `Dangerous`，决策为 `RequireApproval` |
| US01-AC02 | HITL 状态机进入 `Pending` 状态，Agent Loop 暂停，等待人工审批 |
| US01-AC03 | 用户可通过 CLI（`[A]pprove / [R]eject`）或 WebUI（按钮）提交审批决定 |
| US01-AC04 | 用户批准后，`ExecuteApproved()` 校验 `GovernanceAuth`（DecisionID + ActionHash + 未过期）后执行 |
| US01-AC05 | 用户拒绝后，Agent 状态变为 `Rejected`，同一 Action 不可绕过审批再次提交 |
| US01-AC06 | 用户请求 MoreInfo 后，Agent 收到人工反馈，生成新 Action 并重新进入 Governance 评估 |
| US01-AC07 | 审批超时（默认 5 分钟）自动拒绝。HITL decision = rejected，Agent final status = `Failed`，StopCondition = `Timeout` |
| US01-AC08 | 整个决策链路（Risk → Rules → Decision → Human Decision）写入 Audit Log（`audit.jsonl`） |
| US01-AC09 | 上述全部行为可在 mock LLM 下做确定性测试，不依赖真实 LLM |

### 2.2 US-02：测试失败反馈驱动 Agent 自我修正

> **作为** 开发者，
> **我希望** 当 Agent 提交的代码未能通过测试时，Harness 能自动解析测试失败信息，将结构化反馈回灌给 Agent，使其据此修正代码，
> **以便** Agent 不需要我手动告知它"哪里错了"，而是自己从客观反馈中学习并修正。

**Non-goals**：不实现自动证明代码正确，不替代真实测试。仅提供结构化反馈回灌机制。

**验收标准**：

| AC# | 描述 |
|-----|------|
| US02-AC01 | `run_tests` 工具执行后，TestParser 将 `go test -json` 解析为统一的 `Observation` 结构体 |
| US02-AC02 | Shell 执行失败时，ShellParser 将 stderr + exit code 解析为 `Observation` |
| US02-AC03 | `Observation` 作为 `LastObservation` 回灌到 Agent State，在下一轮 Context Assembly 中注入 |
| US02-AC04 | 在 mock LLM 的确定性测试中：控制变量法，same initial state + different Observation → Agent 的下一步 Action 发生变化 |
| US02-AC05 | 反馈不依赖 LLM 自行解读——Parser 是确定性代码，Observation 是结构化数据 |
| US02-AC06 | 连续多次失败后 Agent 达到 `MaxIterations` 停止条件，不会无限循环 |

### 2.3 US-03：跨会话记忆按需注入

> **作为** 开发者，
> **我希望** 在项目开发过程中，我（或 Agent）记录的项目约定和历史决策能够在后续新任务中自动按需提供，而不是每次都将所有记忆全量加载，
> **以便** Agent 能持续遵循我们的约定，且不会因为记忆膨胀而浪费上下文窗口。

**Non-goals**：不实现向量数据库和语义检索。MVP 使用关键词/标签匹配。

**验收标准**：

| AC# | 描述 |
|-----|------|
| US03-AC01 | 用户可通过 `harness memory add --type=convention --tags=go,naming --content="..."` 写入记忆条目 |
| US03-AC02 | 用户可通过 `harness memory list` 查看所有记忆条目（显示 type、tags、content 摘要、创建时间） |
| US03-AC03 | Memory Store 将记忆持久化到 JSON 文件，重启后仍可读取 |
| US03-AC04 | Memory Retriever 根据任务描述中的关键词匹配 `tags` 字段，返回相关记忆条目 |
| US03-AC05 | Agent Loop 在 Context Assembly 阶段调用 `Retrieve(taskContext)`，仅注入匹配到的记忆 |
| US03-AC06 | 验证：当任务与某条记忆不相关时，该记忆不被注入（非全量加载） |
| US03-AC07 | 上述行为可在 mock LLM 下测试：给定已知 Memory Store + 给定任务上下文，断言 Retrieve 返回的条目集合 |

### 2.4 US-04：CLI 完整编码任务执行

> **作为** 开发者，
> **我希望** 通过命令行启动一个完整的 coding task，Harness 能自主完成 Agent Loop 的完整生命周期，并在终端输出可观察的执行过程，
> **以便** 我可以在日常开发中快速使用 Harness。

**Non-goals**：不实现 IDE、文件编辑器、多项目管理。

**验收标准**：

| AC# | 描述 |
|-----|------|
| US04-AC01 | `harness run --task "..."` 启动 Agent Loop，从 `Idle` 状态开始执行 |
| US04-AC02 | 每次迭代在终端输出：Iteration #、Action 类型、Tool 执行摘要、Observation 摘要 |
| US04-AC03 | 当 Governance 触发 `RequireApproval` 时，终端暂停并显示 `[A]pprove / [R]eject / [M]ore info` 交互提示 |
| US04-AC04 | Agent 达到 StopCondition 后，终端输出最终状态（Done/Failed/Rejected）和总迭代次数 |
| US04-AC05 | `harness run` 的退出码与最终状态对应：`Done=0`，`Failed=1`，`Rejected=2`。退出码在代码中定义为命名常量，不散落 `os.Exit(1)` |
| US04-AC06 | 支持 `--max-iterations` 和 `--run-timeout` 参数覆盖默认值 |

> **Timeout 命名规范**：`--run-timeout` 控制 Agent 整体运行超时。`hitl_timeout` 和 `tool_timeout` 在 Config（YAML）中独立配置，三者概念不同。

### 2.5 US-05：WebUI 可观测性与审批

> **作为** 开发者或课程评审者，
> **我希望** 通过浏览器打开一个 Web 界面，查看 Agent 的实时运行状态、Action Trace、Governance 决策链路和审计日志，并能在界面上完成 HITL 审批，
> **以便** 我能直观地理解 Harness 内部机制是如何运作的。

**Non-goals**：不实现 IDE、多用户、WebSocket、复杂 Dashboard、实时协作。

**验收标准**：

| AC# | 描述 |
|-----|------|
| US05-AC01 | `harness serve` 启动 HTTP 服务，浏览器可访问 WebUI |
| US05-AC02 | 页面输入任务描述，点击"Run"后通过 `POST /api/run` 创建任务，获得 `run_id` |
| US05-AC03 | 页面轮询 `GET /api/run/:id`，显示 Agent 状态、迭代次数 |
| US05-AC04 | 页面展示 Action Trace（最近 N 次 Action 的类型和参数摘要） |
| US05-AC05 | 页面展示 Governance 决策链路：Risk 等级、匹配规则、决策原因 |
| US05-AC06 | 当状态为 `WaitingApproval` 时，页面展示危险动作详情和 [Approve] [Reject] [More Info] 按钮 |
| US05-AC07 | 用户点击审批按钮后，通过 `POST /api/approval/:id` 提交决定，Agent 继续执行 |
| US05-AC08 | 页面展示 Audit Log：所有 Governance 决策的完整记录 |
| US05-AC09 | 前端使用 HTML + CSS + Vanilla JS，通过 `go:embed` 嵌入，不依赖外部前端框架 |

---

## 3. 功能规约

### 3.1 Agent 主循环（`internal/runtime/loop.go`）

AgentLoop 是 runtime 包的 composition root——它导入所有 domain 包（agent, governance, tools, llm, feedback, protocol），在运行时组装。

| 功能 | 描述 | 实现 |
|------|------|------|
| 状态初始化 | 从 AgentStateIdle 开始，设置 LoopConfig | `NewAgentLoop(llm, gov, tools, config)` |
| 上下文组装 | 将 system prompt + memory 注入 + messages 组装为 LLM 请求 | `buildSystemPrompt(task)` — 注入匹配的记忆 |
| 主循环 | Think → Decide → Act → Observe 循环 | `Run(ctx, task)` — 最多 MaxIterations 轮 |
| Think 阶段 | 调用 LLM Generate | `llm.Generate(ctx, messages)` |
| Decide 阶段 | 运行 Governance Pipeline | `governance.Evaluate(action)` |
| Act 阶段 | 通过 Governance 后执行工具 | `tool.Validate(action)` → `tool.Execute(action)` |
| Observe 阶段 | 处理工具结果，生成 FeedbackObservation | `feedback.Process(toolName, result)` |
| 停机判断 | FinishReasonStop / Error / MaxIterations | 达到 MaxIterations 返回 error |
| Memory 注入 | 检索相关记忆，注入 System Prompt | `SetMemory(store)` — MinScore=3 阈值过滤 |

**状态转换**（通过 `AgentState.TransitionTo()` 验证）：

```
Idle → Thinking → Executing | AwaitingApproval | Error | Terminated
AwaitingApproval → Executing | Error | Terminated
Executing → Observing | Error | Terminated
Observing → Thinking | Error | Terminated
Error → Terminated
```

每次状态转换记录为 `StateTransition{From, To, Timestamp}`，可通过 `StateTransitions()` 获取转换历史。

### 3.2 LLM 抽象层（`internal/llm/`）

| 功能 | 描述 | 实现 |
|------|------|------|
| Provider 接口 | 统一交互接口 | `Generate(ctx, messages) (Response, error)` |
| Message 类型 | 多角色消息 | `RoleSystem/User/Assistant/Tool` + `ToolCall` + `ToolCallFunction` |
| Response 类型 | LLM 响应 | `Message + FinishReason + Usage` |
| MockProvider | 测试用，支持两种模式 | 序列模式（预定义响应）+ 函数模式（动态响应） |
| OpenAI Provider | 真实 LLM 对接 | 待实现 |

**MockProvider 两种模式**：

**模式一：序列模式（Sequence Mode）** — 按预定义顺序返回响应：
```go
mock := llm.NewMockProvider(script)
```

**模式二：函数模式（Function Mode）** — 根据输入 messages 动态决定响应，用于验证反馈闭环：
```go
mock := llm.NewMockProvider(nil)
mock.SetHandler(func(messages []llm.Message) (llm.Response, error) {
    // 根据 messages 内容动态决定响应
})
```

函数模式是 Demo 2（Governance Interception）和 Demo 3（Feedback Loop）的测试基础。

**为什么需要函数模式**：仅靠序列模式无法证明"不同 Observation → 不同 Action"，因为 MockProvider 可以简单地按顺序返回固定响应，而不检查输入。函数模式允许测试断言 Observation 确实影响了下一轮 Context 的内容。

### 3.3 工具系统（`internal/tools/`）

**MVP 4 个工具**：

| 工具 | 功能描述 | 参数 | 返回值 | 危险等级 |
|------|---------|------|--------|:-------:|
| `shell` | 执行 shell 命令 | `command: string` | `{stdout, stderr, exit_code}` | Dangerous |
| `file_read` | 读取文件内容 | `path: string` | `{content: string}` | Safe |
| `file_write` | 写入/创建文件 | `path: string, content: string` | `{bytes_written: int}` | Suspicious |
| `git` | Git 命令（白名单） | `command: string` | `{output: string}` | Suspicious |

**Tool 接口**：

```go
type Tool interface {
    Name() string
    Description() string
    Execute(action protocol.Action) (ToolResult, error)
    Validate(action protocol.Action) error
}
```

**Tool Registry 架构**：

```go
type Registry struct { /* 内部 map[string]Tool */ }

func NewRegistry() *Registry
func (r *Registry) Register(t Tool) error   // 工具注册
func (r *Registry) Get(name string) (Tool, bool)  // 按名查找
func (r *Registry) List() []Tool            // 列出全部工具
```

**架构不变量**：`Agent NEVER directly invokes Tool.Execute()`。所有工具执行必须经过 Governance 评估与授权。AgentLoop 通过 `tool.Validate(action)` → `governance.Evaluate(action)` → `tool.Execute(action)` 的流程执行工具。

**Git 工具安全设计**：GitTool 使用命令白名单（`status/diff/log/branch/add/commit`），任何不在白名单中的命令直接拒绝，且参数不做 shell 拼接。

**Shell 工具安全设计**：ShellTool 使用 `context.WithTimeout` 强制执行 30 秒超时，防止无限运行的命令。

### 3.4 治理系统（`internal/governance/`）★ 深入维度

**评估管线**（5 层，顺序不可变，短路上报）：

```
Action
   ↓
1. Schema Validation    —— SchemaValidator：Action 结构是否合法、参数是否完整
   ↓  失败 → DecisionDeny，管线终止
2. Risk Classification —— RiskClassifier：Low / Medium / High / Critical
   ↓  Critical → DecisionDeny，管线终止
3. Policy Engine       —— PolicyEngine：规则匹配（SHELL-001、FILE-001 等）
   ↓  规则匹配失败 → DecisionDeny，管线终止
4. Execution Boundary  —— ExecutionBoundary：路径沙箱、资源限制
   ↓  越界 → DecisionDeny，管线终止
5. Execution Control   —— ExecutionController：执行超时、并发控制
   ↓  ShouldEscalate → DecisionEscalate
   ↓
Decision
   ├── Allow
   ├── Deny
   ├── RequireApproval (RiskLevel ≥ High)
   └── Escalate
```

**关键特性：短路上报**。Pipeline 在第一个失败阶段即返回，非全部 5 阶段都执行。PipelineResult.Stages 只包含已执行的阶段。这确保了审计日志中能清晰看到"在哪个阶段被拦截"。

**Risk 四级分类**：

| RiskLevel | 含义 | 默认决策 |
|-----------|------|---------|
| `RiskLevelLow` | 安全操作 | Allow |
| `RiskLevelMedium` | 可疑操作 | Allow（策略引擎可覆盖） |
| `RiskLevelHigh` | 高风险操作 | RequireApproval（HITL） |
| `RiskLevelCritical` | 极度危险 | Deny |

**Tool 的 base risk 与 effective risk**：

| Tool | Base Risk | 示例 Action | EffectiveRisk | 说明 |
|------|:---------:|-------------|:------------:|------|
| `shell` | High | `go test ./...` | Medium | Policy Engine 根据命令内容降级 |
| `shell` | High | `git reset --hard` | High → RequireApproval | 匹配 GIT-002 规则 |
| `shell` | High | `rm -rf /` | Critical → Deny | 匹配 SHELL-001 规则 |
| `file_write` | Medium | `file_write ~/.ssh/id_rsa` | Critical | 路径超出 workspace root |
| `git` | Medium | `git status` | Low | 安全只读命令 |

**规则表**：

| 规则 ID | 类别 | 匹配模式 | 触发决策 |
|---------|------|---------|---------|
| GIT-001 | Git | `git push --force` | RequireApproval |
| GIT-002 | Git | `git reset --hard` | RequireApproval |
| GIT-003 | Git | `git clean -f[d]` | RequireApproval |
| SHELL-001 | Shell | `rm -rf /` 或 `rm -rf ~` | Deny |
| SHELL-002 | Shell | `chmod 777` 或 `chmod -R 777` | RequireApproval |
| FILE-001 | File | 写入 `.git/` 目录内文件 | Deny |
| NET-001 | Network | `curl`、`wget`、`nc` 等外发命令 | RequireApproval |
| PATH-001 | Path | 文件操作路径超出 workspace root | Deny |

**GovernanceAuth — HITL 审批绑定**：

```go
type GovernanceAuth struct {
    DecisionID string    // 决策唯一 ID (UUID)
    ActionHash string    // 基于 canonical 表示的确定性 hash
    ActionID   string    // 绑定到具体 Action
    ExpiresAt  time.Time // 授权过期时间（默认 300 秒，可配置）
}
```

**ActionHash 计算**：使用 `ComputeActionHash()` 函数，基于 Action 的 Type + Payload（key 排序后拼接）计算 FNV-1a 风格 hash。保证同一 Action 的确定性和不同 Action 的区分性。

**审计日志**（Pipeline 内存存储，通过 `Pipeline.AuditLog()` 获取）：

```json
{
  "id": "aud-1723631521000000000",
  "timestamp": "2026-08-14T14:32:01+08:00",
  "action_id": "act-1723631521000000000",
  "decision": "deny",
  "actor": "governance-pipeline",
  "details": {
    "schema": {"passed": true, "reason": "action structure valid"},
    "risk": {"passed": false, "reason": "critical risk: rm -rf /"}
  }
}
```

**敏感信息脱敏**：`AuditLogEntry.RedactSensitive()` 方法对 Details 中的 `api_key`、`token`、`secret`、`password`、`credential` 等字段自动替换为 `"[REDACTED]"`。凭据绝不进入日志。

### 3.5 反馈系统（`internal/feedback/`）

| 功能 | 描述 | 输入 | 输出 |
|------|------|------|------|
| FeedbackProcessor | 统一入口，根据工具类型分发解析 | `toolType: string, result: ToolResult` | `FeedbackObservation` |
| ParseShellResult | 解析 shell 命令执行结果 | `stdout, stderr, success` | `FeedbackObservation` |
| ParseTestOutput | 解析 `go test -json` 输出 | `stdout, stderr, success` | `FeedbackObservation`（含 TestFailureDetail 列表） |

```go
type FeedbackObservation struct {
    // 非导出字段，通过方法访问
    success  bool
    source   string              // "shell" | "go_test" | <tool_type>
    output   string
    errorMsg string
    failures []TestFailureDetail
}
```

关键方法：
- `IsSuccess() bool` / `IsError() bool`
- `Source() string` / `Summary() string`
- `FormatForLLM() string` — 唯一面向 LLM 的输出格式
- `ToObservation() agent.Observation` — 转换为 agent 状态跟踪类型

**设计原则**：非导出字段 + 方法访问确保 LLM 上下文只能通过 `FormatForLLM()` 获取格式化后的内容，而非原始 JSON。

### 3.6 记忆系统（`internal/memory/`）

| 功能 | 描述 | 实现 |
|------|------|------|
| 写入 (`FileStore.Save`) | 持久化记忆条目到 JSON 文件 | `MemoryEntry{ID, Type, Tags, Content, CreatedAt, AccessedAt}` |
| 检索 (`Retriever.Retrieve`) | 根据任务上下文关键词匹配相关记忆 | 全查询命中 10 分、标签命中 8 分、单词命中 2-3 分；MinScore=3 阈值过滤 |
| 列表 (`FileStore.List`) | 列出所有记忆条目 | 从 JSON 文件反序列化全部条目 |
| 删除 (`FileStore.Delete`) | 删除指定记忆条目 | 按 ID 删除并重写 JSON 文件 |

**记忆条目结构**：

```go
type MemoryEntry struct {
    ID         string    `json:"id"`
    Type       string    `json:"type"`       // "convention" | "decision" | "error_pattern"
    Tags       []string  `json:"tags"`       // 例如 ["go", "naming", "testing"]
    Content    string    `json:"content"`
    CreatedAt  time.Time `json:"created_at"`
    AccessedAt time.Time `json:"accessed_at"`
}
```

**检索算法**（MVP，关键词评分）：
- 将任务描述分词，与记忆条目的 `tags` 和 `Content` 进行匹配
- 全查询字符串匹配：10 分
- 标签匹配：每个 8 分
- 内容单词匹配：每个 2 分（短词）或 3 分（长词 ≥5 字符）
- **MinScore=3 阈值**：过滤单内容词匹配（2 分），确保至少一个标签词或两个内容词匹配

**按需注入数据流**：

```
AgentLoop.SetMemory(store)
      ↓
Run() → buildSystemPrompt(task)
      ↓
Retriever.Retrieve(task, topK=3)
      ↓
匹配的 MemoryEntry（score ≥ MinScore）
      ↓
追加到 System Prompt 末尾：
"Relevant context from memory:\n- {content}\n- {content}"
      ↓
不匹配的记忆不被加载
```

**架构边界**：Memory 仅影响 System Prompt，不参与 Governance 决策。确保"记忆不能绕过治理"。

### 3.7 CLI 入口（`internal/cli/` + `cmd/harness/`）

CLI 是 runtime 的薄封装——仅导入 `runtime` 包，不导入任何 domain 包。

| 命令 | 功能 | 说明 |
|------|------|------|
| `harness run <task>` | 同步执行任务 | 启动 AgentLoop，阻塞等待完成 |
| `harness submit <task>` | 异步提交任务 | 通过 TaskManager 提交，返回 task_id |
| `harness status <id>` | 查询任务状态 | 通过 TaskManager.Get 查询 |
| `harness list` | 列出所有任务 | 通过 TaskManager.List 查询 |
| `harness cancel <id>` | 取消任务 | 通过 TaskManager.Cancel 取消 |

**架构约束**：CLI 仅导入 runtime 包，不导入 agent/governance/tools/llm/memory/feedback。这是"CLI 是 runtime client，不是第二套 runtime"的架构约束。

### 3.8 WebUI（`internal/web/`）

**异步执行模型**：

```
HTTP Request POST /tasks
      ↓
Task Manager (internal/runtime/)
      ├── 创建 Task (Pending)
      ├── 启动 goroutine → AgentLoop.Run()
      └── 立即返回 202 Accepted {task_id}
      ↓
HTTP Request GET /tasks/{id}
      ↓
Task Manager 查询 Task 状态
      ↓
返回状态快照 {status, result, ...}
```

Agent 在独立 goroutine 中异步执行，不阻塞 HTTP 请求。HTTP 连接断开不影响 AgentLoop 执行。

**API 端点**：

| 端点 | 方法 | 请求体 | 响应体 | 说明 |
|------|------|--------|--------|------|
| `POST /tasks` | Submit | `{"task": "..."}` | `202 {"task_id": "..."}` | 创建异步任务 |
| `GET /tasks/{id}` | Status | — | `200 {"id", "task", "status", "result", ...}` | 查询任务状态 |
| `GET /tasks` | List | — | `200 [{...}, ...]` | 列出所有任务 |
| `DELETE /tasks/{id}` | Cancel | — | `200 {"status": "cancelled"}` | 取消任务 |

**Context 生命周期分离**：HTTP context（`r.Context()`）仅用于读取请求体。任务 context（`context.Background()`）独立管理，浏览器断开不取消 AgentLoop。

**前端技术**：HTML + CSS + Vanilla JS，通过 `go:embed` 嵌入二进制，不依赖外部前端框架（待实现）。

---

## 4. 非功能性需求

### 4.1 性能

| 需求 | 指标 | 说明 |
|------|------|------|
| Agent Loop 响应 | 每轮迭代 ≤ 30 秒（含 LLM 调用） | 受 LLM API 响应时间主导 |
| WebUI 轮询 | 每 1 秒一次，响应 ≤ 100ms | 仅返回状态快照，不返回历史 |
| 工具执行 | 文件操作 ≤ 1 秒，测试执行 ≤ 30 秒 | 较长操作通过超时控制 |
| 冷启动 | 首次启动（含配置加载）≤ 2 秒 | 不含 LLM 初始化 |

### 4.2 安全

| 需求 | 描述 |
|------|------|
| 凭据安全 | API key 绝不硬编码、绝不进入 Git、绝不进入日志。使用 OS Keychain 作为安全存储，.env 仅作兼容输入源。详见 §7 |
| 治理拦截 | 所有工具执行必须经过 Governance 授权。架构不变量：Agent NEVER directly invokes Tool.Execute() |
| 路径边界 | 所有文件操作限制在 `workspace_root` 内，禁止 `../` 和绝对路径逃逸 |
| 执行超时 | 所有 shell 和测试执行使用 `context.WithTimeout()`，防止无限运行 |
| 环境隔离 | 演示部署时，Agent 仅操作受限的 demo workspace，不接触宿主 OS |
| 审计日志脱敏 | 凭据类字段（token、password、api_key 等）写入日志前自动脱敏 |

### 4.3 可用性

| 需求 | 描述 |
|------|------|
| 引导式启动 | 首次运行 `harness init` 引导用户录入 API key、创建配置文件 |
| 清晰的终端输出 | 每次迭代显示关键信息，不产生噪音 |
| 可观测的 WebUI | 直观展示 Agent 状态、Action Trace、Governance 决策、审计日志 |
| 错误可理解 | LLM 调用失败、工具执行错误等均输出可读的错误信息，而非堆栈跟踪 |

### 4.4 可观测性

| 需求 | 描述 |
|------|------|
| Agent 状态跟踪 | 每次迭代输出当前状态、Action、Observation |
| Governance 审计日志 | 所有治理决策记录到 `audit.jsonl`，包含完整的决策链路 |
| 结构化日志 | 使用 Go `slog` 输出结构化日志（JSON 格式），支持日志级别 |

### 4.5 部署安全边界

公网 WebUI 部署场景下，采用三层隔离架构：

```
Public WebUI (Internet)
      ↓  HTTP API
Harness Core (受限进程)
      ↓  workspace boundary
Restricted Workspace/
    └── demo-project/     ← 所有文件操作、shell 执行均在此目录内
```

- 演示部署时，Config 的 `workspace_root` 指定为隔离的 demo 目录
- Sandbox 的路径边界保证所有文件操作无法逃逸到 `workspace_root` 之外
- 不增加容器级沙箱，但让已有的 workspace boundary 在部署场景中真正生效

---

## 5. 系统架构与领域机制设计

### 5.1 顶层架构

```
┌──────────────────────────────────────────────────────────────────┐
│                       Presentation Layer                         │
│                                                                  │
│   ┌──────────────┐              ┌──────────────────────────┐     │
│   │     CLI      │              │          WebUI           │     │
│   │ (internal/   │              │ (internal/web/)          │     │
│   │  cli/)       │              │                          │     │
│   │ harness run  │              │ POST /tasks              │     │
│   │ harness      │              │ GET  /tasks/{id}         │     │
│   │   submit/    │              │ DELETE /tasks/{id}       │     │
│   │   status/    │              │                          │     │
│   │   list/cancel│              │                          │     │
│   └──────┬───────┘              └───────────┬──────────────┘     │
│          │                                  │                    │
│          │  仅导入 runtime 包                │  仅导入 runtime 包  │
│          └──────────────┬───────────────────┘                    │
└─────────────────────────┼────────────────────────────────────────┘
                          ▼
┌──────────────────────────────────────────────────────────────────┐
│                         Runtime Layer                            │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │              internal/runtime/ (Composition Root)           │  │
│  │                                                             │  │
│  │  ┌─────────────────┐    ┌──────────────────┐               │  │
│  │  │   AgentLoop     │    │   TaskManager    │               │  │
│  │  │                 │    │                  │               │  │
│  │  │ Think → Decide  │    │ Submit/Get/Cancel│               │  │
│  │  │   → Act →       │    │ /List/Wait       │               │  │
│  │  │   Observe       │    │ (goroutine 管理) │               │  │
│  │  └────────┬────────┘    └────────┬─────────┘               │  │
│  │           │                      │                          │  │
│  └───────────┼──────────────────────┼──────────────────────────┘  │
│              │ imports              │ imports                     │
│              ▼                      ▼                             │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │                  Domain Layer (星型拓扑)                    │   │
│  │                                                             │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │   │
│  │  │  agent/  │  │   llm/   │  │  tools/  │  │governance│   │   │
│  │  │ 状态机   │  │ Provider │  │ Registry │  │  ★ 五层  │   │   │
│  │  │ 7 状态   │  │ Message  │  │ 4 工具   │  │  管线    │   │   │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘   │   │
│  │       │             │             │             │          │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────────────────┐     │   │
│  │  │ feedback │  │  memory  │  │      protocol/       │     │   │
│  │  │ 反馈解析 │  │ 文件存储 │  │  ★ 共享类型定义      │     │   │
│  │  │          │  │ 关键词   │  │  Action, ToolResult  │     │   │
│  │  │          │  │ 检索     │  │  ActionStatus        │     │   │
│  │  └──────────┘  └──────────┘  └──────────────────────┘     │   │
│  │                                                             │   │
│  │  所有 domain 包只依赖 protocol + 标准库，互不依赖            │   │
│  └───────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────┐  ┌──────────┐                                     │
│  │ config/  │  │credential│  (空包，接口已定义，待实现)           │
│  └──────────┘  └──────────┘                                     │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

**架构原则**：

1. **protocol 包是唯一的共享依赖**：所有 domain 包（agent/governance/tools/llm/feedback/memory）通过 `protocol` 共享类型（Action, ToolResult, ActionStatus），domain 包之间互不依赖
2. **runtime 是 composition root**：AgentLoop 和 TaskManager 在 runtime 包中导入所有 domain 包，在运行时组装。CLI 和 WebUI 仅导入 runtime 包，不直接访问任何 domain 包
3. **星型拓扑**：`protocol ← agent/governance/tools/feedback/llm/memory`，`runtime → 所有 domain 包`，`cli/web → runtime`

**架构不变量**：
1. Agent NEVER directly invokes Tool.Execute() — 所有工具执行必须经过 Governance 评估
2. Approval is bound to the exact Action being approved — GovernanceAuth 包含 ActionHash
3. Every Governance decision is recorded in Audit Log — Pipeline.recordAudit()
4. CLI/WebUI 不导入 agent/governance/tools/llm/memory/feedback 等 domain 包

### 5.2 领域与机制设计

本节对应 A 文件要求的"领域与机制设计"专节。

#### 5.2.1 领域（Coding）的四个核心机制

| 机制 | 在 Coding 领域中的形态 | 本项目的实现方式 |
|------|----------------------|----------------|
| **动作/工具** | 读写文件、执行 shell、运行构建与测试 | 5 个 MVP 工具（read_file, write_file, list_files, execute_shell, run_tests），通过 `ToolRegistry` 注册与分发 |
| **客观反馈信号** | 测试运行结果、lint 输出、编译错误 | 确定性 Parser 解析 `go test -json` 和 shell 输出，生成结构化 `Observation` 回灌给 Agent |
| **危险动作** | 删除文件、危险 shell 命令、强制 git 操作、对外发布 | 五层治理评估管线自动识别并拦截，HITL 等待人工审批 |
| **记忆** | 项目约定、历史决策、代码库知识 | JSON 文件存储 + 关键词/标签检索，按需注入而非全量载入 |

#### 5.2.2 重点深入维度：治理（Governance）

选择治理作为深入维度的理由：

1. **代码密度最高**：Risk Classification、Policy Engine、Sandbox、HITL、Audit 五个子机制全部由确定性代码实现，不依赖 LLM 智能
2. **确定性测试最自然**：构造一个危险 Action → 断言被拦截，无需 LLM
3. **可演示性最强**：拦截 → 暂停 → 审批 → 执行/拒绝，视觉上清晰完整
4. **最契合课程要求**：A 文件明确建议治理作为深入方向

治理的工程深度体现在：

- **五层评估管线**：顺序不可变的治理流程，每一层职责独立
- **架构不变量**：Agent 不能绕过 Governance 直接调用工具
- **授权绑定**：`GovernanceAuth` 包含 `ActionHash`，防止"批准 A，执行 B"
- **完整审计**：所有决策链路记录到 `audit.jsonl`，可审计、可追溯、可展示
- **敏感信息脱敏**：审计日志自动过滤凭据字段

#### 5.2.3 判定标准：移除真实 LLM 后还能单测验证吗？

| 核心机制 | 是否可脱离 LLM 测试 | 验证方式 |
|---------|:---:|---------|
| Agent Loop 状态机 | ✅ | mock LLM 驱动完整生命周期 |
| 工具分发（ToolRegistry） | ✅ | 直接构造 Action，断言执行结果 |
| 治理评估管线 | ✅ | 构造危险 Action，断言拦截/放行 |
| HITL 状态机 | ✅ | 模拟 Pending→Approve/Reject/Timeout 转换 |
| 反馈解析器 | ✅ | 构造 `go test -json` 输出，断言生成的 Observation |
| 记忆存储与检索 | ✅ | 写入记忆条目，断言检索结果 |
| 配置加载 | ✅ | 构造 YAML，断言解析结果 |
| 凭据存储 | ✅ | mock Store 接口，断言 Set/Get/Delete 行为 |

### 5.3 数据流

```
User Input Task
      ↓
AgentLoop.Run(ctx, task)
      ↓
buildSystemPrompt(task) — 注入相关记忆（Memory Retriever）
      ↓
Messages: [system(+memory), user(task)]
      ↓
┌─ Agent Loop 迭代 ─────────────────────────────────────┐
│                                                        │
│  Think: llm.Generate(ctx, messages)                    │
│         → Response (FinishReasonStop | ToolCalls)      │
│         → 追加 assistant message 到 messages           │
│                                                        │
│  Decide: governance.Pipeline.Evaluate(action)          │
│         ├── Allow          → 继续执行                   │
│         ├── Deny           → 追加错误 tool message      │
│         ├── RequireApproval → HITL 回调                 │
│         └── Escalate       → 追加错误 tool message      │
│                                                        │
│  Act: tool.Validate(action) → tool.Execute(action)     │
│       → ToolResult (Success | Error)                   │
│                                                        │
│  Observe: feedback.Process(toolName, result)            │
│          → FeedbackObservation.FormatForLLM()           │
│          → 追加 tool message 到 messages               │
│                                                        │
│  iterations++                                          │
│  if iterations >= MaxIterations → Error                │
│  if FinishReasonStop → Terminated → return result      │
│                                                        │
└────────────────────────────────────────────────────────┘
      ↓
最终状态: AgentStateTerminated | AgentStateError
      ↓
返回 (result string, error)
```

### 5.4 模块依赖关系

```
protocol/          ← 共享类型定义（Action, ToolResult, ActionStatus）
    ↑
    │ 所有 domain 包只依赖 protocol + 标准库
    │
┌───┴──────────────┬──────────────┬──────────────┬──────────────┐
│                  │              │              │              │
agent/          llm/           tools/        governance/    feedback/
│               │              │              │              │
│ 状态机        │ Provider     │ Registry     │ Pipeline     │ Parser
│ 7 状态        │ Message      │ Tool 接口    │ 5 层管线     │ Observation
│ Action 别名   │ MockProvider │              │ Audit        │
│               │              │              │              │
└───┬───────────┴──────┬───────┴──────┬───────┴──────┬───────┘
    │                  │              │              │
    │            memory/              │              │
    │            文件存储+检索         │              │
    │                                  │              │
    └──────────────────┬───────────────┴──────────────┘
                       │
                       │ runtime 导入所有 domain 包
                       ▼
               runtime/ (Composition Root)
               ├── AgentLoop (主循环)
               └── TaskManager (异步任务)
                       │
                       │ 仅导入 runtime 包
                       │
            ┌──────────┴──────────┐
            │                     │
        cli/ (internal/cli/)   web/ (internal/web/)
        CLI 薄封装             HTTP 服务
            │                     │
            └──────────┬──────────┘
                       ▼
               cmd/harness/main.go
```

**关键依赖规则**：
- `protocol/` 无内部依赖（仅标准库）
- Domain 包（agent/governance/tools/llm/feedback/memory）只依赖 `protocol/` + 标准库，互不依赖
- `runtime/` 导入所有 domain 包 + `protocol/`，是唯一的 composition root
- `cli/` 和 `web/` 仅导入 `runtime/`，不导入任何 domain 包
- `cmd/harness/` 仅导入 `cli/`、`web/`、`runtime/`

---

## 6. 数据模型

> 以下类型定义与 `internal/` 下的实际 Go 代码一致。所有 domain 包通过 `protocol` 包共享核心类型。

### 6.1 AgentState（`internal/agent/state.go`）

7 状态 int 枚举 + 合法转换表 + TransitionTo 验证：

```go
type AgentState int

const (
    AgentStateIdle             AgentState = iota // 初始状态
    AgentStateThinking                           // LLM 生成响应
    AgentStateAwaitingApproval                   // 等待 HITL 审批
    AgentStateExecuting                          // 执行工具
    AgentStateObserving                          // 处理工具结果
    AgentStateError                              // 不可恢复错误
    AgentStateTerminated                         // 循环终止
)
```

**合法状态转换**（通过 `transitionTable` map 约束）：

```
Idle → Thinking
Thinking → Executing | AwaitingApproval | Error | Terminated
AwaitingApproval → Executing | Error | Terminated
Executing → Observing | Error | Terminated
Observing → Thinking | Error | Terminated
Error → Terminated
Terminated → (无)
```

关键方法：`TransitionTo(next AgentState) (AgentState, error)` — 任何状态转换都经过同一验证逻辑。

### 6.2 Action（`internal/protocol/action.go`）

```go
type ActionStatus int

const (
    ActionStatusPending   ActionStatus = iota
    ActionStatusRunning
    ActionStatusCompleted
    ActionStatusFailed
    ActionStatusCancelled
)

type Action struct {
    ID        string         `json:"id"`
    Type      string         `json:"type"`
    Payload   map[string]any `json:"payload,omitempty"`
    Status    ActionStatus   `json:"status"`
    Result    *ToolResult    `json:"result,omitempty"`
    Error     string         `json:"error,omitempty"`
    Timestamp time.Time      `json:"timestamp"`
}
```

关键方法：
- `NewAction(actionType string, payload map[string]any) Action` — 创建 Action 并生成 ID
- `SetStatus(newStatus ActionStatus) error` — 带转换验证的状态变更
- `WithResult(result *ToolResult)` — 附加执行结果

### 6.3 ToolResult（`internal/protocol/result.go`）

```go
type ToolResult struct {
    ActionID  string        `json:"action_id"`
    Success   bool          `json:"success"`
    Data      any           `json:"data,omitempty"`
    Error     string        `json:"error,omitempty"`
    Duration  time.Duration `json:"duration_ns"`
    Timestamp time.Time     `json:"timestamp"`
}
```

关键方法：
- `NewSuccessResult(actionID string, data any, duration time.Duration) ToolResult`
- `NewErrorResult(actionID, errMsg string, duration time.Duration) ToolResult`

### 6.4 Tool 接口（`internal/tools/interface.go`）

```go
type Tool interface {
    Name() string
    Description() string
    Execute(action protocol.Action) (ToolResult, error)
    Validate(action protocol.Action) error
}
```

**架构不变量**：Agent NEVER directly invokes Tool.Execute()。所有工具执行必须经过 Governance 评估。

### 6.5 Registry（`internal/tools/interface.go`）

```go
type Registry struct {
    // 内部 map[string]Tool
}

func NewRegistry() *Registry
func (r *Registry) Register(t Tool) error
func (r *Registry) Get(name string) (Tool, bool)
func (r *Registry) List() []Tool
```

### 6.6 Message 与 LLM 类型（`internal/llm/message.go`）

```go
type Role int
const (
    RoleSystem    Role = iota
    RoleUser
    RoleAssistant
    RoleTool
)

type FinishReason int
const (
    FinishReasonStop      FinishReason = iota
    FinishReasonToolCalls
    FinishReasonLength
    FinishReasonError
)

type ToolCall struct {
    ID       string           `json:"id"`
    Type     string           `json:"type"`
    Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"`
}

type Message struct {
    Role       Role       `json:"role"`
    Content    string     `json:"content"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID string     `json:"tool_call_id,omitempty"`
}

type Response struct {
    Message      Message      `json:"message"`
    FinishReason FinishReason `json:"finish_reason"`
    Usage        Usage        `json:"usage,omitempty"`
}
```

关键方法：
- `NewSystemMessage(content string) Message`
- `NewToolMessage(toolCallID, content string) Message`
- `NewToolCallResponse(content string, toolCalls ...ToolCall) Response`
- `Message.WithToolCall(id, name, arguments string)`

### 6.7 Provider 接口（`internal/llm/provider.go`）

```go
type Provider interface {
    Generate(ctx context.Context, messages []Message) (Response, error)
}
```

MockProvider 支持两种模式：
- **序列模式**：`NewMockProvider(script)` — 按预定义顺序返回响应
- **函数模式**：`mock.SetHandler(fn)` — 根据输入 messages 动态决定响应

### 6.8 Governance 类型（`internal/governance/`）

```go
// RiskLevel 四级风险分类
type RiskLevel int
const (
    RiskLevelLow      RiskLevel = iota
    RiskLevelMedium
    RiskLevelHigh       // → RequireApproval (HITL)
    RiskLevelCritical   // → Deny
)

// GovernanceDecision 四态决策
type GovernanceDecision int
const (
    DecisionAllow           GovernanceDecision = iota
    DecisionDeny
    DecisionRequireApproval
    DecisionEscalate
)

// GovernanceAuth — HITL 审批绑定
type GovernanceAuth struct {
    DecisionID string    `json:"decision_id"`
    ActionHash string    `json:"action_hash"`
    ActionID   string    `json:"action_id"`
    ExpiresAt  time.Time `json:"expires_at"`
}

// StageResult — 单阶段评估结果
type StageResult struct {
    StageName      string    `json:"stage"`
    Passed         bool      `json:"passed"`
    Reason         string    `json:"reason,omitempty"`
    RiskLevel      RiskLevel `json:"risk_level,omitempty"`
    ShouldEscalate bool      `json:"should_escalate,omitempty"`
}

// PipelineResult — 聚合评估结果
type PipelineResult struct {
    Decision  GovernanceDecision `json:"decision"`
    Stages    []StageResult      `json:"stages"`
    Auth      *GovernanceAuth    `json:"auth,omitempty"`
    ActionID  string             `json:"action_id"`
    Timestamp time.Time          `json:"timestamp"`
}
```

### 6.9 AuditLogEntry（`internal/governance/audit.go`）

```go
type AuditLogEntry struct {
    ID        string            `json:"id"`
    Timestamp time.Time         `json:"timestamp"`
    ActionID  string            `json:"action_id"`
    Decision  GovernanceDecision `json:"decision"`
    Actor     string            `json:"actor"`
    Details   map[string]any    `json:"details,omitempty"`
}
```

敏感信息脱敏：`RedactSensitive()` 方法对 `api_key`、`token`、`secret`、`password`、`credential` 等字段自动替换为 `"[REDACTED]"`。

### 6.10 FeedbackObservation（`internal/feedback/feedback.go`）

```go
type FeedbackObservation struct {
    // 非导出字段，通过方法访问
    success  bool
    source   string
    output   string
    errorMsg string
    failures []TestFailureDetail
}
```

关键方法：
- `IsSuccess() bool` / `IsError() bool`
- `Source() string` / `Summary() string`
- `FormatForLLM() string` — 唯一面向 LLM 的输出格式
- `ToObservation() agent.Observation` — 转换为 agent 状态跟踪类型

非导出字段 + 方法访问的设计确保 LLM 上下文只能通过 `FormatForLLM()` 获取格式化后的内容。

### 6.11 MemoryEntry（`internal/memory/entry.go`）

```go
type MemoryEntry struct {
    ID         string    `json:"id"`
    Type       string    `json:"type"`
    Tags       []string  `json:"tags"`
    Content    string    `json:"content"`
    CreatedAt  time.Time `json:"created_at"`
    AccessedAt time.Time `json:"accessed_at"`
}
```

### 6.12 Task 与 TaskStatus（`internal/runtime/task.go`）

```go
type TaskStatus int
const (
    TaskStatusPending    TaskStatus = iota
    TaskStatusRunning
    TaskStatusCompleted
    TaskStatusFailed
    TaskStatusCancelled
)

type Task struct {
    ID        string
    Task      string
    Status    TaskStatus
    Result    string
    Error     string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

**TaskStatus 与 AgentState 是独立的两个维度**：TaskStatus 跟踪外部任务生命周期（5 态），AgentState 跟踪内部 Agent 决策循环（7 态）。互不合并。

### 6.13 LoopConfig（`internal/runtime/loop.go`）

```go
type LoopConfig struct {
    MaxIterations int
    SystemPrompt  string
    ToolTimeout   time.Duration
    HITLTimeout   time.Duration
}
```

### 6.14 持久化文件

| 文件 | 格式 | 位置 | 用途 |
|------|------|------|------|
| `memory.json` | JSON | 项目根目录 `.harness/memory.json` | 记忆持久化 |
| `audit.jsonl` | JSONL | 项目根目录 `.harness/audit.jsonl` | 治理审计日志（Pipeline 内存存储） |
| `config.yaml` | YAML | 项目根目录或 `~/.harness/` | 用户配置（待实现） |

---

## 7. 凭据与分发设计

### 7.1 凭据威胁模型

| 威胁 | 等级 | 缓解措施 |
|------|:---:|---------|
| 凭据硬编码在源码中 | 高 | 代码审查 + 架构不变量：所有 API key 通过 CredentialStore 读取 |
| 凭据被提交到 Git | 高 | .gitignore 包含 `.env`、`config.yaml`（含 key 时）；pre-commit hook 检查 |
| 凭据出现在终端 history | 中 | 使用隐藏输入读取 key，而非命令行参数 |
| 凭据出现在日志中 | 中 | 审计日志脱敏；进程环境变量可在日志中出现的风险在文档中说明 |
| .env 文件明文泄露 | 中 | 文档说明 .env 为明文风险；默认仅作兼容输入源 |
| 进程环境被其他进程读取 | 低 | 使用 OS Keychain 作为主存储，key 仅运行时加载到进程内存 |

### 7.2 凭据存储架构

**分阶段实现**：

| 阶段 | 内容 | 说明 |
|:---:|------|------|
| Phase 1 (MVP) | `CredentialStore` 接口 + `MockStore`（测试用）+ `EnvStore`（环境变量/.env 兼容）+ `redactSensitiveFields()` | 确保接口完整、测试可 mock、日志脱敏功能就绪 |
| Phase 2 (延后) | `KeychainStore`（macOS Keychain / Windows Credential Manager / Linux Secret Service） | 平台安全存储，MVP 后实现 |

**Phase 1 接口定义**：

```go
type CredentialStore interface {
    Set(name, secret string) error
    Get(name string) (string, error)
    Delete(name string) error
    Exists(name string) bool
}
```

**安全策略**：
- **存储**：`KeychainStore` 是默认和推荐的存储方案。`EnvStore` 仅作为兼容输入源，不作为安全存储方案
- **录入**：首次运行 `harness init` 时，通过隐藏输入（不回显字符）引导用户录入 API key，存入 Keychain
- **查看**：`harness credential status` 仅显示"已配置/未配置"，不回显明文
- **更新**：`harness credential set` 重新录入，覆盖旧值
- **清除**：`harness credential delete` 需确认后删除
- **日志**：凭据绝不进入任何日志文件（含审计日志，通过 `redactSensitiveFields()` 保证）

### 7.3 分发形态

**主要形态：静态编译的单文件二进制 + Docker 容器镜像**

#### 7.3.1 原生可执行二进制

| 属性 | 说明 |
|------|------|
| 目标平台 | Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64) |
| 构建方式 | `go build -ldflags="-s -w" -o build/harness ./cmd/harness`（产出在 `build/` 目录） |
| 签名 | MVP 阶段不签名，文档说明首次运行时的系统拦截处理 |
| 单文件 | 所有依赖静态编译，WebUI 前端通过 `go:embed` 嵌入二进制 |
| 构建产出目录 | `build/` — 已加入 `.gitignore`，不提交 Git |

#### 7.3.2 Docker 容器镜像

```dockerfile
# 多阶段构建
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -ldflags="-s -w" -o harness ./cmd/harness

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/harness /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["harness"]
```

#### 7.3.3 分发安全

- 二进制分发时，用户在目标机器上首次运行 `harness init` 引导录入 API key
- Docker 运行时，通过 `-e OPENAI_API_KEY=...` 或挂载 `.env` 文件传入 key（文档说明风险）
- 更好的 Docker 方式：使用 Docker secret 或 volume 挂载 keychain 文件

#### 7.3.4 部署验证

公网 WebUI 部署场景，必须验证 workspace boundary 在实际容器/主机环境中真正生效：

```
docker run \
  --mount type=bind,src=./demo-project,dst=/workspace/demo-project \
  -e WORKSPACE_ROOT=/workspace/demo-project \
  ...
```

**验证方法**：在部署环境中启动 Agent，构造 `read_file ../../../etc/passwd` 或 `write_file /tmp/escape` 等越界操作，断言 Governance 的路径边界规则有效拦截。此验证应作为部署 task 的一部分写入 PLAN。

### 7.4 README 要求

分发部分必须在 README 写清：
1. 获取方式（GitHub 地址、Docker pull 命令、二进制下载链接）
2. 运行命令（`docker run`、`./harness run`）
3. Key 在目标机器上的安全配置方式（`harness init` 引导流程）
4. 已知限制（平台/架构/依赖前提）

---

## 8. 技术选型与理由

### 8.1 编程语言：Go 1.22+

| 需求 | Go 的匹配度 |
|------|:---------:|
| 最低版本 | Go 1.22（Docker 构建使用 `golang:1.22-alpine`） |
| 单二进制分发 | ✅ 原生静态编译，跨平台单文件二进制 |
| 状态机建模 | ✅ 强类型 struct + 显式错误处理，天然适合 AgentState、Action、GovernanceDecision |
| 并发 | ✅ goroutine 原生支持工具执行、WebUI 服务、HITL 异步等待 |
| 确定性测试 | ✅ 标准 testing 包，不依赖外部运行时 |
| HTTP 基础设施 | ✅ 标准库 net/http 足够，无需第三方框架 |
| 嵌入前端资源 | ✅ `go:embed` 编译时嵌入 WebUI 静态文件 |
| 结构化日志 | ✅ Go 1.21 内置 `slog`，无需外部库 |

### 8.2 LLM 供应商

| 属性 | 选择 |
|------|------|
| 默认供应商 | OpenAI 兼容 API（含 OpenAI、Azure OpenAI、本地 ollama 等） |
| 协议 | Chat Completions API |
| 模型 | 默认 `gpt-4o`（可在 Config 中配置 `model` 字段） |
| 选型理由 | 生态最成熟、兼容性最广。通过同一接口可对接 OpenAI、Anthropic（通过 API 网关）、本地 ollama 等 |

### 8.3 外部库

| 库 | 用途 | 理由 |
|----|------|------|
| 标准库 `net/http` | HTTP 服务 + CLI | 零外部依赖，WebUI 功能需求足够简单 |
| 标准库 `testing` | 单元测试 | 所有核心机制都需要 mock LLM 测试 |
| 标准库 `slog` | 结构化日志 | Go 1.21 内置，零依赖 |
| 标准库 `embed` | 嵌入前端资源 | Go 1.16 内置，零依赖 |
| 标准库 `context` | 超时控制 | 所有工具执行和 LLM 调用都使用 context 控制超时 |
| 无外部 Agent 框架 | — | 完全自己实现 Agent Loop、治理、反馈、记忆等核心机制 |

### 8.4 被排除的选型

| 技术 | 排除理由 |
|------|---------|
| LangChain / AutoGen / CrewAI | A 文件明确禁止寄生于高层 Agent 编排框架 |
| React / Vue / Vite | WebUI 功能足够简单，HTML + CSS + Vanilla JS 即可 |
| WebSocket | 轮询满足课程展示需求，WebSocket 增加复杂度 |
| 向量数据库 | 记忆 MVP 使用关键词/标签匹配，不引入向量检索 |
| 容器级沙箱（Docker-in-Docker / gVisor） | MVP 排除，路径边界 + 超时 + 环境白名单已足够 |

---

## 9. 验收标准

### 9.1 功能验收

| 模块 | 验收标准 | 关联 US |
|------|---------|:------:|
| Agent Loop | 可在 mock LLM 下完整运行 5 轮迭代，达到 5 种 StopCondition 中的至少 3 种 | US-04 |
| LLM Provider | MockProvider 可预定义响应序列；RealProvider 可连接真实 API | US-01,02,03,04 |
| Tool Registry | 5 个工具均可注册、校验、执行；未注册工具返回错误 | US-01,02,04 |
| Governance | 五层管线可评估 Action；危险命令被拦截；审计日志可写入 | US-01 |
| HITL | 四种状态转换（Approve/Reject/MoreInfo/Timeout）均正确 | US-01 |
| Feedback | TestParser 和 ShellParser 可解析对应输出为 Observation | US-02 |
| Memory | 写入、检索、列表功能正常；检索结果按需而非全量 | US-03 |
| CLI | 5 个顶层命令均可使用：`run`, `serve`, `init`, `memory` (add/list), `credential` (status/set/delete) | US-04 |
| WebUI | 3 个 API 端点正常；页面展示状态、Action Trace、HITL 审批 | US-05 |

### 9.2 测试验收

| 标准 | 要求 |
|------|------|
| 核心机制确定性测试 | 所有核心机制（Governance、Tool Registry、Feedback Parser、Memory Store/Retriever、Config、Credential、Audit）均有不依赖真实 LLM 的确定性单元测试 |
| Mock LLM 集成测试 | Agent Loop 及需要决策行为的集成测试使用 Mock LLM 驱动，不调用真实 LLM API |
| 测试不依赖网络 | 所有测试不调用真实 LLM API |
| 一键运行 | `make test` 或 `go test ./...` 一键运行全部测试 |
| CI 通过 | 最后一次 CI 执行必须为 pass 状态 |
| 机制演示 | 三个演示场景（见 §9.3）可在 mock LLM 下确定性地复现 |

### 9.3 机制演示（对应 A 文件 §A.6）

| 演示 | 描述 | 关联 US | 测试方式 |
|------|------|:-------:|---------|
| ① 治理护栏拦截危险动作 | 构造 `execute_shell` Action 含 `git reset --hard`，Governance 拦截为 `RequireApproval` | US-01 | 直接构造 Action → 调用 Governance.Evaluate → 断言 Decision = RequireApproval |
| ② 反馈闭环改变 Agent 行为 | 控制变量法：same initial state + same Mock LLM decision script + different Observation（Success:true vs Success:false）→ Agent 下一步 Action 不同 | US-02 | mock LLM 脚本固定，仅注入不同 Observation → 断言 Context 中 Observation 影响了下轮 Action |
| ③ 治理完整决策链路可审计 | 提交审批决策后，审计日志包含完整决策链路（Risk → Rules → Human Decision） | US-01, US-05 | 构造完整流程 → 断言 `audit.jsonl` 包含正确的审计记录 |

### 9.4 分发验收

| 标准 | 要求 |
|------|------|
| Docker 构建 | 单条 `docker build` 成功，单条 `docker run` 可启动 |
| 二进制构建 | `make build` 产出单文件二进制 |
| 平台支持 | Linux (amd64) + macOS (amd64) 至少通过构建 |
| CI 构建 | CI 中产出可下载的构建产物（鼓励） |

### 9.5 凭据验收

| 标准 | 要求 |
|------|------|
| 无硬编码 | 源码中无任何真实 API key |
| 无 Git 历史泄露 | .gitignore 覆盖 `.env`、`config.yaml`（含 key 的情况） |
| 安全存储 | OS Keychain 可用 |
| 引导式录入 | `harness init` 引导用户录入 key |
| 日志无凭据 | 审计日志执行 `redactSensitiveFields()` |

---

## 10. 风险与未决问题

### 10.1 风险清单

| 风险 | 概率 | 影响 | 缓解措施 |
|------|:---:|:---:|---------|
| LLM API 调用不稳定（超时/限流/不可用） | 中 | 高 | MockProvider 保障测试不依赖网络；RealProvider 有明确的错误类型分类，调用层决定是否重试 |
| 治理规则引擎过于简单，无法覆盖真实场景 | 中 | 中 | 规则引擎支持可配置和扩展；MVP 内置 5 条规则，覆盖最常见场景 |
| ActionHash 实现细节（canonicalJSON）导致序列化不一致 | 低 | 中 | 定义明确的 canonicalJSON 规范（key 按字典序排序），并有对应的测试 case |
| WebUI 轮询导致性能问题 | 低 | 低 | 轮询间隔可配置；仅返回状态快照，不返回历史 |
| 跨平台 Keychain 接口差异 | 中 | 中 | 接口抽象（CredentialStore），各平台实现独立；.env 作为 fallback |
| **Go 版本环境差异**（开发环境 Go 版本与 SPEC 要求不一致） | 中 | 低 | SPEC 明确最低版本 Go 1.22；Docker 构建锁定 `golang:1.22-alpine`；本地开发使用系统 Go 但确保兼容性 |
| **make 依赖**（Windows 无预装 make） | 中 | 中 | Makefile 提供构建自动化；文档中写明 `make` 为可选依赖，提供等价 `go` 命令作为 fallback（`go test ./...`、`go build ./cmd/harness`） |
| **项目根目录名称约定**（首次克隆用户不确定目录名） | 低 | 低 | 文档明确：项目根目录名为 `CageHarness`，与 GitHub 仓库名一致 |
| 冷启动验证暴露 SPEC 缺陷 | 高 | 高 | 这是正常流程，冷启动验证的目的就是发现缺陷，在实现前修正 |

### 10.2 未决问题

| 问题 | 状态 | 计划决策时间 |
|------|------|------------|
| canonicalJSON 的完整规范（嵌套对象、null 处理） | 实现时确定 | PLAN 阶段 |
| WebUI 的 CSS 框架选择 | 不做框架，纯手写 CSS | 已决定 |
| 是否需要 pre-commit hook 检查凭据泄露 | 建议但非 MVP | Phase 2 |
| 测试覆盖率目标 | 核心机制 ≥ 80%，整体 ≥ 60% | CI 配置时确定 |
| Memory 检索的匹配阈值 | 实现时通过测试确定 | PLAN 阶段 |
| 是否需要支持多语言测试框架（非 go test） | MVP 仅 go test，后续扩展 | 已决定 |

### 10.3 已知限制

| 限制 | 说明 |
|------|------|
| 单 Agent 执行 | 不支持多 Agent 并行或协作 |
| 单 LLM 供应商 | 仅 OpenAI 兼容 API，不原生支持多供应商 |
| 单项目 | 一次运行管理一个 Agent 任务 |
| 无容器级沙箱 | 路径边界 + 超时 + 环境白名单，非 OS 级隔离 |
| Memory 无向量检索 | 关键词/标签匹配，非语义检索 |
| 前端无框架 | HTML + CSS + Vanilla JS，不支持复杂交互 |
| 无实时推送 | HTTP 轮询 (~1s)，非 WebSocket |

---

## 附录：模块清单

| 类别 | 模块 | 路径 | 职责 |
|------|------|------|------|
| Shared | Protocol | `internal/protocol/` | 共享类型定义（Action, ToolResult, ActionStatus）— 所有 domain 包的唯一公共依赖 |
| Core | Agent | `internal/agent/` | 7 状态 AgentState + 合法转换表 + Action 别名 + Observation 类型 |
| Core | LLM | `internal/llm/` | Provider 接口 + Message/Response 类型 + MockProvider（序列模式+函数模式） |
| Core | Tools | `internal/tools/` | Tool 接口 + Registry + ShellTool + FileTool + GitTool（安全白名单） |
| Core | Governance ★ | `internal/governance/` | 五层 Pipeline（Schema→Risk→Policy→Boundary→Control）+ Audit + GovernanceAuth |
| Core | Feedback | `internal/feedback/` | FeedbackProcessor + ParseShellResult + ParseTestOutput + FeedbackObservation |
| Core | Memory | `internal/memory/` | FileStore（JSON 持久化）+ Retriever（关键词评分 + MinScore 阈值） |
| Core | Runtime | `internal/runtime/` | Composition Root：AgentLoop（主循环）+ TaskManager（异步任务） |
| Core | Config | `internal/config/` | 配置加载（接口已定义，待实现） |
| Core | Credential | `internal/credential/` | 凭据安全存储（接口已定义，待实现） |
| Presentation | CLI | `internal/cli/` | CLI 薄封装（Run/Submit/Status/List/Cancel），仅导入 runtime |
| Presentation | WebUI | `internal/web/` | HTTP 服务（POST/GET/DELETE /tasks），仅导入 runtime |
| Entry | CMD | `cmd/harness/` | CLI 入口（5 个命令） |

---

## 附录：项目目录结构

> 项目根目录名：`CageHarness/`（与 GitHub 仓库名一致）
> 以下为根目录下的实际文件结构（与代码一致）：

```
CageHarness/
├── cmd/
│   └── harness/
│       └── main.go                 # CLI 入口（5 个命令）
├── internal/
│   ├── protocol/                   # 共享类型定义（所有 domain 包的唯一公共依赖）
│   │   ├── action.go               #   Action, ActionStatus, NewAction
│   │   ├── result.go               #   ToolResult, NewSuccessResult, NewErrorResult
│   │   └── doc.go                  #   包文档
│   ├── agent/                      # Agent 状态机
│   │   ├── state.go                #   AgentState（7 状态 + 合法转换表 + TransitionTo）
│   │   ├── action.go               #   Action 类型别名（→ protocol.Action）
│   │   ├── observation.go          #   Observation 类型
│   │   ├── state_test.go
│   │   ├── action_test.go
│   │   ├── observation_test.go
│   │   └── doc.go
│   ├── llm/                        # LLM 抽象层
│   │   ├── provider.go             #   Provider 接口 + ProviderFunc
│   │   ├── message.go              #   Message, Response, ToolCall, Role, FinishReason
│   │   ├── mock.go                 #   MockProvider（序列模式 + 函数模式）
│   │   ├── openai.go               #   OpenAI Provider（待实现）
│   │   ├── provider_test.go
│   │   ├── message_test.go
│   │   ├── openai_test.go
│   │   └── doc.go
│   ├── tools/                      # 工具系统
│   │   ├── interface.go            #   Tool 接口 + Registry（Register/Get/List）
│   │   ├── shell.go                #   ShellTool（命令执行 + 30s 超时）
│   │   ├── file.go                 #   FileTool（读写文件 + 路径沙箱校验）
│   │   ├── governed.go             #   GitTool（安全 git 命令白名单）
│   │   ├── result.go               #   ToolResult（工具内部使用）
│   │   ├── interface_test.go
│   │   ├── shell_test.go
│   │   ├── file_test.go
│   │   ├── governed_test.go
│   │   ├── result_test.go
│   │   └── doc.go
│   ├── governance/                 # 治理（★ 深入维度）
│   │   ├── pipeline.go             #   Pipeline（五层评估管线 + Evaluate/ApproveHITL/RejectHITL）
│   │   ├── schema.go               #   SchemaValidator（第 1 层）
│   │   ├── types.go                #   RiskLevel, GovernanceAuth, StageResult, PipelineResult, GovernanceContext
│   │   ├── audit.go                #   GovernanceDecision, AuditLogEntry, RedactSensitive
│   │   ├── policy.go               #   PolicyEngine（第 3 层：规则匹配）
│   │   ├── boundary.go             #   ExecutionBoundary（第 4 层：路径沙箱）
│   │   ├── pipeline_test.go
│   │   ├── types_test.go
│   │   ├── audit_test.go
│   │   └── doc.go
│   ├── feedback/                   # 反馈闭环
│   │   ├── feedback.go             #   FeedbackProcessor, ParseShellResult, ParseTestOutput, FeedbackObservation
│   │   ├── feedback_test.go
│   │   └── doc.go
│   ├── memory/                     # 记忆
│   │   ├── entry.go                #   MemoryEntry 类型
│   │   ├── store.go                #   FileStore（JSON 文件持久化）
│   │   ├── retriever.go            #   Retriever（关键词评分检索 + MinScore 阈值）
│   │   ├── entry_test.go
│   │   ├── store_test.go
│   │   ├── retriever_test.go
│   │   └── doc.go
│   ├── runtime/                    # 运行时（Composition Root）
│   │   ├── loop.go                 #   AgentLoop（主循环：Think→Decide→Act→Observe）
│   │   ├── task.go                 #   TaskManager（异步任务：Submit/Get/Cancel/List/Wait）
│   │   ├── loop_test.go
│   │   ├── task_test.go
│   │   └── doc.go
│   ├── cli/                        # CLI 薄封装
│   │   ├── cli.go                  #   CLI（Run/Submit/Status/List/Cancel），仅导入 runtime
│   │   ├── cli_test.go
│   │   └── doc.go
│   ├── web/                        # WebUI HTTP 服务
│   │   ├── web.go                  #   Server（POST/GET/DELETE /tasks），仅导入 runtime
│   │   ├── web_test.go
│   │   └── doc.go
│   ├── config/                     # 配置（待实现）
│   │   └── doc.go
│   └── credential/                 # 凭据安全（待实现）
│       └── doc.go
├── tests/
│   └── demo/
│       └── phase13_demo_test.go    # 5 个 Demo 测试
├── web/static/                     # WebUI 静态资源（待实现）
├── build/                          # 构建产物（.gitignore）
├── config.example.yaml             # 配置模板
├── .env.example                    # 环境变量模板
├── Dockerfile                      # 容器构建
├── Makefile                        # 构建脚本
├── .gitlab-ci.yml                  # CI/CD 配置
├── go.mod
├── go.sum
├── .gitignore
├── README.md
├── SPEC.md                         # 本文档
├── PLAN.md
├── SPEC_PROCESS.md
├── AGENT_LOG.md
└── REFLECTION.md
```