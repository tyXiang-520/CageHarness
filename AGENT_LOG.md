# AGENT_LOG.md · 开发日志

> 按时间顺序记录关键节点。每条包含：时间戳、task 编号、使用的技能/工作流、关键 prompt/context 配置、subagent 输出或 commit hash、人工干预记录、学到的教训。

---

## 方法论说明

本项目使用 **Claude Code** 作为编码智能体，工作流遵循 Superpowers 方法论的核心纪律：

- **TDD**：红-绿-重构，先写测试，确认失败，再写实现
- **git worktree 隔离**：每个 Phase 在独立 worktree 中开发
- **subagent 驱动**：复杂任务派发 Explore/Plan 子 agent 并行搜索
- **Gate Review**：每个 Phase 完成后由 ChatGPT 独立审阅，逐条判断采纳
- **ChatGPT 作为外部审阅者**：每轮迭代后用户将代码变更转发给 ChatGPT 获取审阅意见

| 角色 | 工具 | 职责 |
|------|------|------|
| 主开发 agent | Claude Code (Opus 4.8) | 编写代码、运行测试、管理 worktree |
| 外部审阅者 | ChatGPT | 每阶段 Gate Review，发现架构问题 |
| 人类决策者 | 用户（向天宇） | 转发审阅意见、判断采纳/推翻、最终签字 |

---

## Phase 0：项目脚手架

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 上午 |
| Commit | `00ff87a` |
| 技能 | Explore agent（搜索现有仓库结构），Write（创建文件） |
| 内容 | 初始化 go.mod、创建目录结构、Makefile、Dockerfile、.gitignore、.gitlab-ci.yml、config.example.yaml、.env.example、cmd/harness/main.go、各 internal 包的 doc.go |

**人工干预**：无。

**教训**：Phase 0 的脚手架测试（cold_start_test.go）在后续 Phase 中持续扩展，这证明了"先建骨架、再填内容"的策略有效。

---

## Phase 1：Core Types

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 上午 |
| Commit | `dc3f9f1` |
| 技能 | TDD（先写测试），Write（实现） |
| 内容 | AgentState 状态机（7 状态 + 合法转换表 + TransitionTo）、Protocol 类型（Action, ToolResult, Tool 接口）、包文档 |

**人工干预**：无。

**关键设计决策**：AgentState 的合法转换通过 `transitionTable` map 约束，而非散落在代码各处。这确保了状态机在任何地方调用 `TransitionTo` 时都经过同一验证逻辑。

**教训**：在 Phase 1 就定义好状态机，为后续 Phase 7（AgentLoop）省了大量时间。

---

## Phase 2：LLM Layer

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 上午 |
| Commit | `8b39f0c` |
| 技能 | TDD，Write |
| 内容 | Message/Response/ToolCall 类型、Provider 接口、MockProvider（支持序列模式和函数模式）、HITLResponse 占位 |

**人工干预**：无。

**关键设计决策**：MockProvider 的函数模式（`SetHandler`）是后续所有测试的基础。没有它，Demo 2（Governance Interception）和 Demo 3（Feedback Loop）都无法实现。

**教训**：在 LLM 层就设计好 mock 抽象，是 A 项目"移除真实 LLM 后机制仍可单测"这一硬性要求的前提。

---

## Phase 5：Tools

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 下午 |
| Commit | `c241ab3` |
| 技能 | TDD，Write |
| 内容 | Registry（工具注册与查找）、ShellTool（命令执行 + 30s 超时）、FileTool（读写文件 + 路径沙箱校验）、TestTool（go test 运行）、GitTool（安全 git 命令白名单 + 参数校验） |

**人工干预**：无。

**关键设计决策**：GitTool 使用命令白名单（status/diff/log/branch/add/commit），任何不在白名单中的命令直接拒绝，且参数不做 shell 拼接。

**教训**：工具的 Validate 方法和 Execute 方法分离，允许 Governance 在 Validate 阶段就能拦截。

---

## Phase 6：Governance ★（核心深度维度）

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 下午 |
| Commit | `66af54c` → `76bf354` → `4a867e5` |
| 技能 | TDD，Write，Gate Review |
| 内容 | 五层评估管线（Schema → Risk → Policy → Boundary → Control）、RiskLevel 四级（Low/Medium/High/Critical）、GovernanceDecision 三态（Allow/Deny/RequireApproval）、GovernanceAuth + ActionHash（HITL 审批绑定）、PipelineResult 审计结构 |

**人工干预**：
1. ChatGPT 审阅发现 RiskClassifier 的 `shouldEscalate := risk == RiskLevelHigh` 导致 HITL（RequireApproval）不可达。修正为 `shouldEscalate := false`，将 Escalation 保留给 ExecutionController。
2. Gate Review 修正了架构描述：从 `agent → governance → tools` 改为 `protocol ← agent/governance/tools`。

**关键设计决策**：Pipeline 短路上报（第一个失败阶段即返回），非全部 5 阶段都执行。这在 Demo 4 审计输出中体现——只有 2 个阶段被记录（Schema 通过 + Risk 失败）。

**教训**：多轮审阅对 Governance 的价值最大——从 V1 的三层设计进化到 V2 的五层+审计，每次审阅都发现了新的工程深度。

---

## Phase 7：Agent Loop

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 下午 |
| Commit | `6e648d6` |
| 技能 | TDD，Write |
| 内容 | AgentLoop 主循环（Think → Decide → Act → Observe）、LoopConfig、HITLHandler 回调、StateTransition 记录、tool call 参数解析 |

**人工干预**：
1. `llm.NewUserMessage` 不存在，改为 `llm.NewMessage(llm.RoleUser, task)`。
2. `decision.Reason()` 不存在，改为 `governanceDenyReason()` 辅助函数遍历 Stages。

**关键设计决策**：AgentLoop 是 runtime 包的 composition root——它导入所有 domain 包（agent, governance, tools, llm, feedback, protocol），但 domain 包之间互不依赖。这解决了循环依赖问题。

**教训**：Runtime 作为"胶水层"的设计值得保留——它将 domain 包之间的依赖关系转化为星型拓扑，每个 domain 包只依赖 protocol 和标准库。

---

## Phase 8：Feedback

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 下午 |
| Commit | `9868c75` → `49859d4` |
| 技能 | TDD，Write，Gate Review |
| 内容 | FeedbackProcessor、FeedbackObservation（success/source/output/error/failures）、FormatForLLM()、ParseShellResult()、ParseTestOutput()（go test -json）、AgentLoop 集成（executeToolCall 中调用 feedback.Process） |

**人工干预**：
1. ToolResult 使用 `Data`（any）而非 `Output`（string），且无 `ExitCode` 字段。Feedback 解析器改用 `dataToString()` 辅助函数。
2. Gate Review 后冻结了 20+ 个公开 API。

**关键设计决策**：FeedbackObservation 使用非导出字段 + 方法访问，确保 LLM 上下文只能通过 `FormatForLLM()` 获取格式化后的内容，而非原始 JSON。

**教训**：API Freeze 在 Phase 8 执行是合理的——此时核心循环已完成，后续 Phase（Memory、TaskManager、CLI、WebUI）都是消费者而非修改者。

---

## Phase 9：Memory

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 下午 |
| Commit | `119e181` → `2794d2e` |
| 技能 | TDD，Write，Gate Review |
| 内容 | MemoryEntry（+ AccessedAt 字段）、FileStore（JSON 文件持久化）、Retriever（关键词评分检索：全查询 10 分、标签 8 分、单词 2-3 分）、AgentLoop.SetMemory() + buildSystemPrompt() |

**人工干预**：
1. Gate Review 发现缺少注入限制——新增 `Retriever.MinScore=3` 阈值，过滤单内容词匹配（2 分）。
2. 重复测试名（store_test.go 与 entry_test.go 冲突），移除重复。

**关键设计决策**：Memory 仅影响 System Prompt（通过 buildSystemPrompt），不参与 Governance 决策。这确保了"记忆不能绕过治理"的架构边界。

**教训**：Memory 最容易膨胀成 RAG 系统。Phase 9 严格控制在"关键词评分 + top-k 注入"的 MVP 范围内，没有引入向量数据库或语义搜索。

---

## Phase 10：TaskManager

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 下午 |
| Commit | `4321484` |
| 技能 | TDD，Write |
| 内容 | TaskStatus 五态（Pending/Running/Completed/Failed/Cancelled）、Task 结构体、TaskManager（Submit/Get/Cancel/List/Wait）、sync.RWMutex + atomic 并发安全 |

**人工干预**：无。

**关键设计决策**：TaskStatus 与 AgentState 是独立的两个维度——Task 包含 AgentLoop 执行，AgentLoop 有 AgentState。互不合并。

**教训**：这是项目第一个大量使用 goroutine 的模块。虽然 Windows 环境无法运行 race detector（缺少 gcc），但通过 copy-on-read（Get/List 返回副本）和 RWMutex 保护 map 的方式，代码结构上避免了数据竞争。

---

## Phase 11：CLI

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 下午 |
| Commit | `50ec38e` |
| 技能 | TDD，Write |
| 内容 | CLI 薄封装（Run/Submit/Status/List/Cancel）、cmd/harness/main.go 入口（5 个命令）、TestCLI_IsRuntimeClient 架构验证 |

**人工干预**：无。

**关键设计决策**：CLI 仅导入 runtime 包，不导入 agent/governance/tools/llm/memory/feedback。这是"CLI 是 runtime client，不是第二套 runtime"的架构约束。

**教训**：CLI 的每一层都清晰可追溯：`main.go → CLI → TaskManager → AgentLoop → Governance → Tools`。没有跳层调用。

---

## Phase 12：WebUI

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 下午 |
| Commit | `a034b94` |
| 技能 | Pre-Gate Review（4 项检查），TDD，Write |
| 内容 | HTTP Server（POST/GET/DELETE 路由）、context.Background() 任务生命周期、九项测试覆盖全部 Gate |

**人工干预**：
1. go vet 发现 3 处 `t.Fatalf` 格式字符串参数不匹配，逐一修正。
2. 二进制文件 `harness.exe` 被误提交，通过 `git rm --cached` + `.gitignore` 修正。

**关键设计决策**：HTTP context（`r.Context()`）仅用于读取请求体，任务 context（`context.Background()`）独立管理。浏览器断开不取消 AgentLoop。

**教训**：WebUI 是最容易破坏架构边界的模块。四个 Pre-Gate 检查（依赖隔离、异步模型、Context 分离、无新状态机）在实现前就锁定了红线。

---

## Phase 13：Integration + Demo

| 项目 | 内容 |
|------|------|
| 时间 | 2026-08-14 下午 |
| Commit | `1583e3b` |
| 技能 | TDD，Write |
| 内容 | 5 个 Demo 测试：Cold Start、Governance Interception（ExecutionCount==0）、Feedback Loop、Audit Trace（JSON 输出）、End-to-End Integration |

**人工干预**：
1. Demo 1 的 `go test ./...` 递归运行了自身导致超时，改为 `go test ./internal/...`。
2. Demo 2 的断言逻辑修正——Governance 拦截后 AgentLoop 优雅处理（LLM 放弃并返回最终响应），不抛出 error。
3. Demo 4 的 Pipeline 短路上报——只有 2 个阶段被记录（非 5 个），修正断言。

**关键设计决策**：Demo 2 的 `countingTool.ExecutionCount() == 0` 是区分"治理拦截"与"简单错误处理"的关键断言。工具从未被执行，而不仅仅是返回了 Deny 决策。

**教训**：Demo 测试不能只验证"系统返回了什么"，必须验证"系统没有做什么"。ExecutionCount==0 比 Decision==Deny 更能证明治理边界。

---

## 总体统计

| 指标 | 数值 |
|------|------|
| 总 Phase | 14 |
| 总 commit | 16 |
| 人工干预次数 | 11 次（全部记录于各 Phase 条目） |
| 采纳 ChatGPT 审阅建议 | 约 40+ 条 |
| 推翻 ChatGPT 审阅建议 | 3 条（保留 WebUI、分阶段 Credential、升级五层治理） |
| 零外部依赖 | ✅ go.mod 仅含标准库 |
| 测试通过率 | 100%（12 个包，全部 PASS） |

---

## 最有效的 prompt/context 策略

1. **ChatGPT 作为外部审阅者**：每 Phase 完成后将代码变更和架构决策转发给 ChatGPT，获取独立审阅。这是本项目最重要的质量保障机制——ChatGPT 发现的架构问题（HITL 不可达、全量注入、API 类型不匹配）在开发阶段就被修正，而非积累到集成阶段。
2. **Worktree 隔离 + 独立 Phase 分支**：每个 Phase 在独立 worktree 中开发，互不干扰。Phase 6 的 Governance 重构和 Phase 5 的 Tools 开发可以并行进行。
3. **Gate Review 机制**：在 Phase 6、8、9、12 前执行 Gate Review，将"架构边界检查"作为独立步骤而非事后审计。
4. **TDD 红-绿-重构**：先写测试、确认失败、再写实现。在 Phase 10（TaskManager 并发）和 Phase 12（WebUI Context 分离）中，测试先行避免了并发 bug 和生命周期问题。

## 如果重做会改变什么

1. **Phase 3（Config）和 Phase 4（Credential）应更早实现**：目前 Config 和 Credential 是空包，导致 CLI 入口使用硬编码的 MockProvider。如果 Phase 3/4 在 Phase 7（AgentLoop）之前完成，整个测试链可以使用真实配置。
2. **Phase 5（Tools）的 GitTool 白名单应更严格**：当前白名单包含 `add` 和 `commit`，但缺少对 `--force` 等危险 flag 的校验。
3. **Phase 12（WebUI）应使用 httptest 的更多能力**：当前测试使用 `httptest.NewServer`，但未利用其 `Close` 和 `Wait` 机制做更精细的 lifecycle 测试。