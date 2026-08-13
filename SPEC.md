# SPEC · Coding Agent Harness

> **Spec-Driven, Subagent-Built, Human-Owned.**
>
> 项目：AI4SE 期末项目 — A · Coding Agent Harness
> 技术栈：Go
> 深入维度：治理（Governance — 护栏 + 沙箱 + HITL + 审计）
> 版本：V2.2（最终冻结）

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

### 3.1 Agent 主循环（`internal/agent/`）

| 功能 | 描述 | 输入 | 输出 | 边界条件 |
|------|------|------|------|---------|
| 状态初始化 | 创建 AgentState，设置 Goal 和初始 Status=Idle | 任务描述字符串、配置 | AgentState 实例 | 任务描述为空时返回错误 |
| 上下文组装 | 将 system prompt、messages、memory 条目、observation 组装为 LLM 请求 | AgentState + Memory 条目 | Message 列表 | 超过上下文窗口限制时截断早期消息 |
| 主循环迭代 | 1→2→3→4→5→6→1 循环执行 | 当前 AgentState | 更新后的 AgentState | 达到 StopCondition 时退出循环 |
| 停机判断 | 检查 5 种 StopCondition | 当前 AgentState | StopCondition + 最终 Status | 多个条件同时满足时按优先级：Completed > HumanRejected > Unrecoverable > Timeout > MaxIterations |
| 退出码映射 | 将最终 Status 映射为进程退出码 | AgentStatus | ExitCode 常量 | 统一映射，不散落 `os.Exit(N)` |

### 3.2 LLM 抽象层（`internal/llm/`）

| 功能 | 描述 | 输入 | 输出 | 边界条件 |
|------|------|------|------|---------|
| Chat 接口 | 统一交互接口 | `[]Message` | `(Response, error)` | 网络超时、API 错误、空响应均返回 error |
| OpenAI Provider | 对接 OpenAI 兼容 API | `[]Message` + API key + 端点配置 | `(Response, error)` | 认证失败、速率限制、模型不可用均返回明确的 error 类型 |
| Mock Provider | 脚本驱动，预定义响应序列 | `[]Message` + `ResponseScript` | `(Response, error)` | 脚本耗尽时返回 error；可配置中间失败 |

**Mock Provider 设计**：

MockProvider 需要支持两种模式，以满足不同测试场景：

**模式一：序列模式（Sequence Mode）** — 按预定义顺序返回响应，适合简单场景：

```go
type MockProvider struct {
    script    ResponseScript
    callCount int
}

type ResponseScript struct {
    Responses []ScriptedResponse
}

type ScriptedResponse struct {
    Content string
    Error   error  // nil 表示正常响应，非 nil 表示模拟 LLM 失败
}
```

**模式二：函数模式（Function Mode）** — 根据输入 messages 动态决定响应，用于验证反馈闭环：

```go
// MockFunc 接收 messages 并返回响应
// 可用于验证 Observation 是否确实进入了下一轮 Context
type MockFunc func(messages []Message) (Response, error)

type MockProviderFunc struct {
    fn MockFunc
}
```

**为什么需要函数模式**：仅靠序列模式无法证明"不同 Observation → 不同 Action"，因为 MockProvider 可以简单地按顺序返回固定响应，而不检查输入。函数模式允许测试断言 Observation 确实影响了下一轮 Context 的内容。详细说明见 §9.3 机制演示②。

### 3.3 工具系统（`internal/tools/`）

**MVP 5 个工具**：

| 工具 | 功能描述 | 参数 | 返回值 | 危险等级 |
|------|---------|------|--------|:-------:|
| `read_file` | 读取文件内容 | `path: string` | `{content: string}` | Safe |
| `write_file` | 写入/创建文件 | `path: string, content: string` | `{bytes_written: int}` | Suspicious |
| `list_files` | 列出目录内容 | `path: string` | `{files: string[]}` | Safe |
| `execute_shell` | 执行 shell 命令 | `command: string, cwd?: string` | `{stdout: string, stderr: string, exit_code: int}` | Dangerous |
| `run_tests` | 执行测试套件 | `pattern?: string, path?: string` | `{pass: bool, failures: TestFailure[], output: string}` | Suspicious |

**Tool Registry 架构**：

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() []ParamSchema
    Execute(ctx context.Context, auth GovernanceAuth, action Action) (ToolResult, error)
    // Execute 只接受 Governance 颁发的授权，不接受裸调用
}

type ToolRegistry struct {
    tools map[string]Tool
}

func (r *ToolRegistry) Validate(action Action) error
    // 校验：工具是否存在、参数是否符合 schema
func (r *ToolRegistry) ExecuteApproved(ctx context.Context, action Action, auth GovernanceAuth) (ToolResult, error)
    // 执行：校验 auth + 校验 action hash + 校验过期 → 调用 tool.Execute
```

**架构不变量**：`Agent NEVER directly invokes Tool.Execute()`。所有工具执行必须经过 Governance 评估与授权。

### 3.4 治理系统（`internal/governance/`）★ 深入维度

**评估管线**（顺序不可变）：

```
Action
   ↓
1. Schema Validation    —— Action 结构是否合法、参数是否完整
   ↓
2. Risk Classification —— Safe / Suspicious / Dangerous
   ↓
3. Policy Engine       —— 规则匹配（命令/路径/Git/文件规则）
   ↓
4. Execution Boundary  —— 路径边界、环境白名单、网络策略
   ↓
5. Execution Control   —— 策略检查：该 Action 允许的超时/取消策略是否有效
                         执行时强制：Tool.Execute() 使用 context.WithTimeout() 强制执行
   ↓
6. Decision
   ├── Allow
   ├── Reject
   └── RequireApproval
   ↓
7. HITL State Machine  —— Pending → Approved / Rejected / MoreInfo
   ↓
8. Audit Log           —— 记录完整决策链路
```

**Tool 的 base danger level 与 effective risk**：

Tool 的 `DangerLevel` 是其 base risk（默认分类），但最终的 EffectiveRisk 由 Governance 根据完整的 Action（tool + params）综合计算：

| Tool | Base Risk | 示例 Action | EffectiveRisk | 说明 |
|------|:---------:|-------------|:------------:|------|
| `execute_shell` | Dangerous | `go test ./...` | Suspicious | Policy Engine 根据命令内容降级 |
| `execute_shell` | Dangerous | `git reset --hard` | Dangerous → RequireApproval | 匹配 GIT-002 规则 |
| `execute_shell` | Dangerous | `rm -rf /` | Reject | 匹配 SHELL-001 规则 |
| `write_file` | Suspicious | `write_file ~/.ssh/id_rsa` | Dangerous | 路径超出 workspace root |
| `run_tests` | Suspicious | `run_tests ./...` | Allow | 常规测试运行 |

**规则**：Tool 的 base DangerLevel 提供默认值；Policy Engine 可根据 Action 内容覆盖或细化风险等级。这保证了 `execute_shell` 作为能力分类是 Dangerous，但 `go test ./...` 不会不必要地触发 HITL。

| 规则 ID | 类别 | 匹配模式 | 触发决策 |
|---------|------|---------|---------|
| GIT-001 | Git | `git push --force` | RequireApproval |
| GIT-002 | Git | `git reset --hard` | RequireApproval |
| GIT-003 | Git | `git clean -f[d]` | RequireApproval |
| SHELL-001 | Shell | `rm -rf /` 或 `rm -rf ~` | Reject |
| SHELL-002 | Shell | `chmod 777` 或 `chmod -R 777` | RequireApproval |
| FILE-001 | File | 写入 `.git/` 目录内文件 | Suspicious |
| NET-001 | Network | `curl`、`wget`、`nc` 等外发命令 | Suspicious |
| PATH-001 | Path | 文件操作路径超出 workspace root | Reject |

**HITL 状态机**：

```
                      ┌───────────┐
                      │  Pending  │
                      └─────┬─────┘
                  ┌─────────┼──────────┐
                  ▼         ▼          ▼
            ┌─────────┐ ┌─────────┐ ┌───────────┐
            │Approved │ │Rejected │ │ MoreInfo  │
            └────┬────┘ └────┬────┘ └─────┬─────┘
                 │           │            │
                 ▼           ▼            ▼
           ┌──────────┐ ┌────────┐ ┌──────────────┐
           │Executing │ │Agent   │ │Re-enter Gov  │
           └────┬─────┘ │Stop    │ │(携带原始     │
                │       └────────┘ │Action +      │
                ▼                 │humanFeedback)│
           ┌──────────┐           └──────────────┘
           │  Done    │
           │ / Failed │
           └──────────┘
```

**关键约束**：
- Rejected 后同一 Action 不可绕过审批再次提交
- MoreInfo 携带 `originalAction + humanFeedback` 重新进入 Governance 评估管线
- Timeout 默认 5 分钟，可配置；超时自动拒绝（HITL decision = rejected，Agent final status = Failed）

**GovernanceAuth**：

```go
type GovernanceAuth struct {
    DecisionID string    // 决策唯一 ID (UUID)
    ActionHash string    // SHA256(canonicalJSON(action)) — 绑定到具体 Action
    ExpiresAt  time.Time // 授权过期时间（默认 30 秒）
}
```

**ActionHash 计算规范**：基于规范化后的 Action 表示计算，保证字段顺序等序列化差异不会导致同一 Action 得到不同 hash。禁止直接使用 `json.Marshal(map[string]any)`（map key 顺序不确定）。使用 `canonicalJSON` 序列化，保证 key 按字典序排序，字段值递归规范化。

**审计日志**：

```json
{
  "timestamp": "2026-08-13T14:32:01+08:00",
  "action": "execute_shell",
  "params": {
    "command": "git reset --hard HEAD~1"
  },
  "risk": "dangerous",
  "decision": "require_approval",
  "matched_rules": ["GIT-002"],
  "reasons": [
    "git reset --hard is irreversible",
    "operation modifies project commit history"
  ],
  "human_decision": "rejected",
  "human_reason": "current branch has unpushed commits"
}
```

**敏感信息脱敏**：写入审计日志前，对 `params` 中的敏感字段（`token`、`password`、`api_key`、`authorization`、`secret` 等）执行 `redactSensitiveFields()` 替换为 `"***REDACTED***"`。凭据绝不进入日志。

### 3.5 反馈系统（`internal/feedback/`）

| 功能 | 描述 | 输入 | 输出 |
|------|------|------|------|
| TestParser | 解析 `go test -json` 输出 | `go test -json` 的 stdout | `Observation{Success, Source:"go_test", ErrorType, Details}` |
| ShellParser | 解析 shell 执行结果 | `{stdout, stderr, exit_code}` | `Observation{Success, Source:"shell", ErrorType, Details}` |
| Observation 统一类型 | 结构化反馈载体 | — | 见下方定义 |

```go
type Observation struct {
    Success   bool
    Source    string      // "go_test" | "shell" | "go_build" (deferred) | "lint" (deferred)
    ErrorType string      // "" (成功时) | "test_failure" | "build_error" | "shell_error"
    Details   interface{} // 结构化的具体信息
}
```

### 3.6 记忆系统（`internal/memory/`）

| 功能 | 描述 | 输入 | 输出 |
|------|------|------|------|
| 写入 (`Store`) | 持久化记忆条目 | `{Type, Tags, Content}` | `{ID, CreatedAt}` |
| 检索 (`Retrieve`) | 根据任务上下文匹配相关记忆 | `taskContext: string` | `[]MemoryEntry`（仅匹配的条目） |
| 列表 (`List`) | 列出所有记忆条目 | 无 | `[]MemoryEntrySummary`（摘要信息） |

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

**检索算法**（MVP，关键词/标签匹配）：将任务描述分词，与记忆条目的 `tags` 和 `Content` 进行子串匹配。匹配分数高于阈值的条目返回。不投入向量检索。

**按需注入数据流**：

```
Memory Store (JSON file)
      ↓
Retriever.Retrieve(taskContext)
      ↓
Matching MemoryEntries only
      ↓
Agent Loop → Context Assembly → 注入匹配的记忆
      ↓
不匹配的记忆不被加载
```

### 3.7 CLI 入口（`cmd/harness/`）

| 命令 | 功能 | 参数 | 说明 |
|------|------|------|------|
| `harness run` | 运行编码任务 | `--task`, `--max-iterations`, `--run-timeout` | 启动 Agent Loop，终端输出完整生命周期 |
| `harness serve` | 启动 WebUI 服务 | `--port` | 启动 HTTP 服务，提供 WebUI |
| `harness init` | 初始化项目 | 无 | 引导用户创建配置文件、录入 API key |
| `harness memory add` | 添加记忆条目 | `--type`, `--tags`, `--content` | 写入 Memory Store |
| `harness memory list` | 列出记忆条目 | 无 | 显示所有记忆摘要 |

### 3.8 WebUI

**异步执行模型**：

```
HTTP Request POST /api/run
      ↓
Task Manager (internal/runtime/)
      ├── 创建 TaskRecord
      ├── 启动 goroutine → Agent.Run()
      └── 立即返回 {run_id}
      ↓
HTTP Request GET /api/run/:id
      ↓
Task Manager 查询 TaskRecord 状态
      ↓
返回状态快照 {status, iteration, ...}
```

Agent 在独立 goroutine 中异步执行，不阻塞 HTTP 请求。HITL 等待期间，HTTP 连接不会挂起。

**API 端点**：

| 端点 | 方法 | 请求体 | 响应体 | 说明 |
|------|------|--------|--------|------|
| `POST /api/run` | Create | `{"task": "..."}` | `{"run_id": "..."}` | 创建新任务，返回 run_id |
| `GET /api/run/:id` | Read | — | 完整状态快照（见下方） | 轮询 Agent 状态 |
| `POST /api/approval/:id` | Approve | `{"decision": "approve"\|"reject"\|"more_info", "reason": "..."}` | `{"status": "ok"}` | 提交审批决定 |

**`GET /api/run/:id` 响应体**：

```json
{
  "status": "running",
  "iteration": 3,
  "current_action": {
    "tool": "execute_shell",
    "params": {...}
  },
  "observations": [],
  "governance": {
    "risk": "dangerous",
    "decision": "require_approval",
    "matched_rules": ["GIT-002"],
    "reasons": ["..."],
    "waiting_approval": true
  },
  "audit": [
    {"timestamp": "...", "action": "...", "decision": "..."}
  ]
}
```

**前端技术**：HTML + CSS + Vanilla JS，通过 `go:embed` 嵌入二进制，不依赖外部前端框架。HTTP 轮询（~1s 间隔），不使用 WebSocket。

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
│   │ harness run  │              │ harness serve            │     │
│   │ harness init │              │ Observability + HITL     │     │
│   └──────┬───────┘              └───────────┬──────────────┘     │
│          │                                  │                    │
└──────────┼──────────────────────────────────┼────────────────────┘
           │                                  │
           └──────────────┬───────────────────┘
                          ▼
┌──────────────────────────────────────────────────────────────────┐
│                         Harness Core                             │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                        Agent Loop                          │  │
│  │                                                             │  │
│  │  State → Context Assembly → LLM → Action Parser →           │  │
│  │       Governance → Tool Dispatch → Observation →           │  │
│  │       State Update → Stop Condition Check                   │  │
│  │                                                             │  │
│  │  架构不变量                                                  │  │
│  │  (1) Agent NEVER directly invokes Tool.Execute()            │  │
│  │  (2) Approval is bound to the exact Action being approved   │  │
│  │  (3) Every Governance decision is recorded in Audit Log     │  │
│  └────────────────────────────────────────────────────────────┘  │
│              │           │            │            │              │
│              ▼           ▼            ▼            ▼              │
│  ┌───────────┐  ┌───────────┐  ┌────────────┐  ┌───────────┐    │
│  │    LLM    │  │   Tools   │  │Governance ★│  │ Feedback  │    │
│  │  Real +   │  │ Registry  │  │            │  │           │    │
│  │   Mock    │  │           │  │┌──────────┐│  │ TestParser│    │
│  └───────────┘  │ Validate()│  ││Risk      ││  │ShellParser│    │
│                 │Execute    │  ││Policy    ││  │           │    │
│                 │ Approved()│  ││Exec Bndry││  │      →    │    │
│                 └───────────┘  ││Exec Ctrl ││  │Observation│    │
│                                ││HITL      ││  └───────────┘    │
│              ┌───────────┐    ││Audit Log ││        │          │
│              │  Memory   │    │└──────────┘│        │          │
│              │Store+     │    └────────────┘        │          │
│              │Retrieve   │                          │          │
│              └─────┬─────┘                          │          │
│                    │                                │          │
│              ┌─────▼──────┐            ┌───────────▼──────────┐  │
│              │   Config   │            │     Credential       │  │
│              │ YAML Load  │            │  Secure Store (主)   │  │
│              └────────────┘            │  .env (兼容输入源)   │  │
│                                        └──────────────────────┘  │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

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
Agent Loop (State Initialization)
      ↓
Context Assembly (System Prompt + Messages + Memory + LastObservation)
      ↓
LLM Call (Chat → Response)
      ↓
Action Parser (Response → Action)
      ↓
Governance Pipeline
  ├── Allow          → ToolRegistry.ExecuteApproved()
  ├── Reject         → AgentState update (StopCondition = HumanRejected)
  └── RequireApproval → HITL Pending → Human Decision → ExecuteApproved | Reject
      ↓
Tool Execution → ToolResult
      ↓
Feedback Parser → Observation
      ↓
Agent State Update (Iteration++, LastObservation = Observation)
      ↓
Stop Condition Check
  ├── No  → 下一轮迭代 (Context Assembly)
  └── Yes → 返回最终 Status
```

### 5.4 模块依赖关系

```
agent
  ├── llm        (调用 Chat)
  ├── tools      (调用 ToolRegistry.ExecuteApproved)
  ├── governance (调用 Evaluate, ExecuteApproved 时传递 auth)
  ├── feedback   (解析工具结果)
  ├── memory     (Context Assembly 时调用 Retrieve)
  └── config     (读取配置)

cli
  └── agent      (启动 Agent.Run)
  └── credential (引导 key 录入)
  └── memory     (memory add/list 命令)

web
  └── agent      (启动 Agent.Run, 查询状态)
  └── governance (HITL 审批)

credential
  └── config     (读取 Secure Store 配置)

config
  └── (无依赖，纯文件加载)
```

---

## 6. 数据模型

### 6.1 AgentState

```go
type AgentStatus string
const (
    StatusIdle            AgentStatus = "idle"
    StatusRunning         AgentStatus = "running"
    StatusWaitingApproval AgentStatus = "waiting_approval"
    StatusDone            AgentStatus = "done"
    StatusFailed          AgentStatus = "failed"
    StatusRejected        AgentStatus = "rejected"
)

type StopCondition string
const (
    StopCompleted          StopCondition = "completed"
    StopMaxIterations      StopCondition = "max_iterations"
    StopUnrecoverableError StopCondition = "unrecoverable_error"
    StopHumanRejected      StopCondition = "human_rejected"
    StopTimeout            StopCondition = "timeout"
)

type AgentState struct {
    Goal            string
    Iteration       int
    Messages        []Message
    PendingAction   *Action
    LastObservation *Observation
    Status          AgentStatus
    StopCondition   StopCondition
}
```

### 6.2 StopCondition → Status 映射

| StopCondition | Final Status |
|--------------|-------------|
| `Completed` | `Done` |
| `MaxIterations` | `Failed` |
| `UnrecoverableError` | `Failed` |
| `HumanRejected` | `Rejected` |
| `Timeout` | `Failed` |

### 6.3 Message

```go
type Message struct {
    Role    string // "system" | "user" | "assistant"
    Content string
}
```

### 6.4 Action

```go
type Action struct {
    Tool   string         `json:"tool"`
    Params map[string]any `json:"params"`
}
```

### 6.5 GovernanceAuth

```go
type GovernanceAuth struct {
    DecisionID string    `json:"decision_id"`
    ActionHash string    `json:"action_hash"`
    ExpiresAt  time.Time `json:"expires_at"`
}
```

### 6.6 GovernanceDecision

```go
type DecisionType string
const (
    DecisionAllow          DecisionType = "allow"
    DecisionReject         DecisionType = "reject"
    DecisionRequireApproval DecisionType = "require_approval"
)

type GovernanceDecision struct {
    Decision     DecisionType
    Risk         DangerLevel
    Reasons      []string
    MatchedRules []string
    Auth         *GovernanceAuth // 非 nil 的两种情况：
                                 // (1) Decision=Allow 时立即签发
                                 // (2) Decision=RequireApproval 且人工审批通过后签发
}
```

### 6.7 Observation

```go
type Observation struct {
    Success   bool
    Source    string      // "go_test" | "shell"
    ErrorType string      // "" | "test_failure" | "shell_error"
    Details   interface{} // 结构化详情
}

// TestFailure 详情
type TestFailureDetail struct {
    TestName string
    Message  string
}

// ShellError 详情
type ShellErrorDetail struct {
    ExitCode int
    Stderr   string
}
```

### 6.8 ToolResult

```go
type ToolResult struct {
    Success  bool
    Stdout   string
    Stderr   string
    ExitCode int
    Data     map[string]any // 工具特定的结构化数据
}
```

### 6.9 MemoryEntry

```go
type MemoryEntry struct {
    ID         string    `json:"id"`
    Type       string    `json:"type"`       // "convention" | "decision" | "error_pattern"
    Tags       []string  `json:"tags"`
    Content    string    `json:"content"`
    CreatedAt  time.Time `json:"created_at"`
    AccessedAt time.Time `json:"accessed_at"`
}
```

### 6.10 AuditLogEntry

```go
type AuditLogEntry struct {
    Timestamp     time.Time              `json:"timestamp"`
    Action        string                 `json:"action"`
    Params        map[string]any         `json:"params"`
    Risk          string                 `json:"risk"`
    Decision      string                 `json:"decision"`
    MatchedRules  []string               `json:"matched_rules,omitempty"`
    Reasons       []string               `json:"reasons,omitempty"`
    HumanDecision string                 `json:"human_decision,omitempty"`
    HumanReason   string                 `json:"human_reason,omitempty"`
}
```

### 6.11 Config

```go
type Config struct {
    LLM struct {
        Endpoint  string  `yaml:"endpoint"`
        Model     string  `yaml:"model"`
        MaxTokens int     `yaml:"max_tokens"`
        Timeout   Duration `yaml:"timeout"`
    } `yaml:"llm"`

    Agent struct {
        MaxIterations int      `yaml:"max_iterations"`
        RunTimeout    Duration `yaml:"run_timeout"`
    } `yaml:"agent"`

    Governance struct {
        Enabled      bool     `yaml:"enabled"`
        HITLTimeout  Duration `yaml:"hitl_timeout"`
        ToolTimeout  Duration `yaml:"tool_timeout"`
        WorkspaceRoot string  `yaml:"workspace_root"`
        Rules        []string `yaml:"rules"` // 启用/禁用的规则 ID 列表
    } `yaml:"governance"`

    Web struct {
        Port int `yaml:"port"`
    } `yaml:"web"`
}
```

### 6.12 退出码常量

```go
const (
    ExitCodeDone     = 0
    ExitCodeFailed   = 1
    ExitCodeRejected = 2
)
```

### 6.13 持久化文件

| 文件 | 格式 | 位置 | 用途 |
|------|------|------|------|
| `config.yaml` | YAML | 项目根目录或 `~/.harness/` | 用户配置 |
| `memory.json` | JSON | 项目根目录 `.harness/memory.json` | 记忆持久化 |
| `audit.jsonl` | JSONL | 项目根目录 `.harness/audit.jsonl` | 治理审计日志 |

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
| Core | Agent Loop | `internal/agent/` | 状态机 + 上下文组装 + 停机判断 + Action 类型 |
| Core | LLM | `internal/llm/` | Provider 接口 + Real + Mock |
| Core | Tools | `internal/tools/` | 5 个工具 + Registry + Validate/ExecuteApproved |
| Core | Governance ★ | `internal/governance/` | Risk + Policy + ExecBoundary + ExecControl + HITL + AuditLog |
| Core | Feedback | `internal/feedback/` | 2 个 Parser → Observation |
| Core | Memory | `internal/memory/` | JSON Store + 标签检索 |
| Core | Runtime | `internal/runtime/` | Task Manager：异步任务创建、状态跟踪、goroutine 管理 |
| Core | Config | `internal/config/` | YAML 加载 + 校验 |
| Core | Credential | `internal/credential/` | Secure Store + 兼容输入 |
| Presentation | WebUI | `web/` | Thin layer：Observability + HITL |
| Entry | CLI | `cmd/harness/` | CLI 入口 |

---

## 附录：项目目录结构

> 项目根目录名：`CageHarness/`（与 GitHub 仓库名一致）
> 以下为根目录下的完整结构：

```
harness/
├── cmd/
│   └── harness/
│       └── main.go              # CLI 入口
├── internal/
│   ├── agent/                   # Agent 主循环
│   │   ├── state.go             #   AgentState, AgentStatus, StopCondition
│   │   ├── loop.go              #   主循环状态机
│   │   ├── context.go           #   上下文组装
│   │   ├── action.go            #   Action 类型（Agent↔Tool 协议）
│   │   ├── loop_test.go
│   │   └── state_test.go
│   ├── llm/                     # LLM 抽象层
│   │   ├── interface.go         #   Provider 接口
│   │   ├── openai.go            #   OpenAI 实现
│   │   ├── mock.go              #   Mock 实现
│   │   ├── mock_test.go
│   │   └── types.go             #   Message, Response
│   ├── tools/                   # 工具系统
│   │   ├── registry.go          #   注册表
│   │   ├── tool.go              #   Tool 接口
│   │   ├── file.go              #   文件工具
│   │   ├── shell.go             #   Shell 工具
│   │   ├── test.go              #   测试工具
│   │   ├── registry_test.go
│   ├── governance/              # 治理（★ 深入维度）
│   │   ├── evaluator.go         #   评估管线
│   │   ├── risk.go              #   风险分类
│   │   ├── policy.go            #   策略引擎 + 规则
│   │   ├── boundary.go          #   执行边界（路径/环境/网络）
│   │   ├── control.go           #   执行控制（超时）
│   │   ├── hitl.go              #   HITL 状态机
│   │   ├── auth.go              #   GovernanceAuth
│   │   ├── audit.go             #   审计日志
│   │   ├── decision.go          #   GovernanceDecision 类型
│   │   ├── evaluator_test.go
│   │   ├── hitl_test.go
│   │   ├── policy_test.go
│   │   ├── audit_test.go
│   │   └── auth_test.go
│   ├── feedback/                # 反馈闭环
│   │   ├── test_parser.go       #   go test 解析器
│   │   ├── shell_parser.go      #   Shell 解析器
│   │   ├── observation.go       #   Observation 类型
│   │   ├── test_parser_test.go
│   │   └── shell_parser_test.go
│   ├── memory/                  # 记忆
│   │   ├── store.go             #   存储接口 + JSON 实现
│   │   ├── retriever.go         #   检索器
│   │   ├── entry.go             #   MemoryEntry 类型
│   │   ├── store_test.go
│   │   └── retriever_test.go
│   ├── runtime/                  # 运行时（Task Manager）
│   │   └── task.go               #   Task 创建、状态跟踪、goroutine 管理
│   ├── config/                  # 配置
│   │   ├── config.go            #   加载 + 校验
│   │   └── config_test.go
│   └── credential/              # 凭据安全
│       ├── store.go             #   CredentialStore 接口
│       ├── keychain.go          #   OS Keychain 实现
│       ├── env.go               #   环境变量兼容实现
│       ├── store_test.go
│       └── redact.go            #   敏感信息脱敏
├── web/                         # WebUI
│   ├── server.go                #   HTTP 服务
│   ├── handler.go               #   API 路由
│   └── static/                  #   前端资源（go:embed）
│       ├── index.html
│       ├── style.css
│       └── app.js
├── config.example.yaml          # 配置示例
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
├── .gitignore
├── .gitlab-ci.yml
├── README.md
├── SPEC.md
├── PLAN.md
├── SPEC_PROCESS.md
├── AGENT_LOG.md
└── REFLECTION.md
```