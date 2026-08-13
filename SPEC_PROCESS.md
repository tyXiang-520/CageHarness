# SPEC_PROCESS · 过程文档

> **Spec-Driven, Subagent-Built, Human-Owned.**
>
> 记录与 Superpowers 协作生成 SPEC 与 PLAN 的全过程，以及冷启动验证结果。

---

## 一、Brainstorming 关键节点

### 1.1 第一轮：核心定位

| 节点 | 内容 | 决策 |
|------|------|------|
| 定位选择 | 教学型 + 实验型深度拓展 vs 实用型 vs 平台型 | 选择"教学型基底 + 治理深度" |
| 目标用户 | 课程评审老师为主 + 本人开发为辅 | 确定 |
| 编程语言 | TypeScript / Go / Rust / Python | 选择 Go |
| 深入维度 | 治理（护栏 + HITL）vs 记忆架构 | 选择治理，理由：代码密度最高、确定性测试最自然、可演示性最强 |

**决策依据**：教学型基底确保代码结构高度透明，契合 AI4SE 课程展示；治理维度天然由代码构成，深入实现后最能体现工程深度，符合 A 文件"主角维度做深"的要求。

### 1.2 第二轮：系统架构与四类机制

| 节点 | 内容 | 决策 |
|------|------|------|
| 架构层次 | CLI + WebUI 双入口，共享 Harness Core | 确定 |
| 四类机制 | 动作/工具、反馈信号、危险动作、记忆 | 全部定义 |
| 治理深度 | 三层：DangerLevel → Rule → HITL | 初始设计，后续升级为五层 |
| 模块划分 | 8 个核心模块 + 1 个 presentation 层 | 确定 |

### 1.3 第三轮：Architecture Review 后的 V2 修正

ChatGPT 对 V1 架构进行正式审阅，提出了 16 条意见。逐条判断后采纳了 13 条、确认了 3 条。最重要的修正：

| 修正 | 变更前 | 变更后 |
|------|--------|--------|
| Agent Loop 引入 State | 流水线式循环 | 状态机驱动 + 5 种 StopCondition |
| Action 抽象层 | Tool 自带 DangerLevel | Action 独立，Governance 判断 Action 内容 |
| Memory 按需检索 | 全量注入 | 标签检索 + 按需注入 |
| Credential 模块 | 未设计 | CredentialStore 接口 + Keychain + .env |
| WebUI 分层 | `internal/web/` | `web/` 与 `internal/` 同级，不侵入 Core |

### 1.4 第四轮：V2.1 → V2.2 冻结

| 冻结项 | 决策 |
|--------|------|
| 架构不变量 | ① Agent NEVER directly invokes Tool.Execute() ② Approval is bound to exact Action ③ Every Governance decision is recorded |
| GovernanceAuth | DecisionID + ActionHash + ExpiresAt |
| ActionHash | canonicalJSON 序列化 + SHA256 |
| 审计日志 | JSONL 格式，存储于 `.harness/audit.jsonl` |
| WebUI API | 3 个端点，HTTP 轮询，无 WebSocket |
| 5 个工具 | read_file, write_file, list_files, execute_shell, run_tests |
| 2 个 Parser MVP | go test + shell result，build/lint 延后 |

---

## 二、关键迭代（至少 3 轮）

### 迭代 1：从"流水线 Loop"到"状态机 Loop"

**背景**：V1 架构中，Agent Loop 被描述为"Context Assembly → LLM → Parse → Execute → Feedback"的流水线，缺少显式的状态管理和停机判断。

**AI 建议**：ChatGPT 审阅指出 "Agent Loop 需要显式引入 State 与 Stop Condition"，并给出了 `AgentState` 结构体建议。

**处理决策**：采纳。这是 V1 中最严重的架构缺失。A 文件明确要求 loop 须包含"停机判断"，且核心机制必须能在 mock LLM 下做确定性测试——没有显式状态机，这两条都无法满足。

**对话节选**：
> 审阅："A 文件明确把'停机'列为核心机制，而且核心机制需要在 mock LLM 条件下可测试。因此请将 loop 模块重新设计为真正的状态驱动 Agent Loop，而不是简单的函数串联。"
>
> 采纳后新增：`AgentState{Goal, Iteration, Messages, PendingAction, LastObservation, Status}` + 5 种 `StopCondition`

### 迭代 2：从"全量注入"到"按需检索"

**背景**：V1 的 Memory 设计是"启动时全量加载 JSON，并全部注入 System Prompt"。这在 A 文件审阅中被指出直接违反了"按需提供而非全量载入"的要求。

**AI 建议**：ChatGPT 审阅和后续审阅都指出"全量注入"不符合要求，建议改为 JSON 文件 + 关键词/标签检索。

**处理决策**：采纳。这是严重程度最高的修正之一。全量注入在项目约定较少时可行，但一旦记忆条目超过数十条，上下文窗口会迅速膨胀，且注入无关记忆会降低 Agent 决策质量。

**修订效果**：记忆系统改为 `Store → Retriever(taskContext) → Matching entries only → Context Assembly` 的数据流，非匹配的记忆不被加载。

### 迭代 3：从"三层治理"到"五层治理 + 审计"

**背景**：V1 的治理设计为三层（DangerLevel → Rule → HITL），随着审阅推进，逐步升级为五层评估管线 + 审计日志。

**AI 建议**：多轮审阅逐步建议：
1. 增加 Action 抽象层，让 Governance 判断 Action 内容而非 Tool 本身
2. 增加 Sandbox / Execution Boundary
3. 分离 Execution Control（超时是执行控制，不是沙箱）
4. 增加 GovernanceAuth 和 ActionHash，实现"审批绑定到具体 Action"
5. 增加审计日志，实现可审计性

**处理决策**：全部采纳。治理是本项目的深入维度，每一层增加都直接提升了工程深度。从最终结果看，治理从 V1 的"能拦截危险动作"进化到 V2.2 的"五层评估管线 + HITL 状态机 + 审计日志"，实现了"分类、解释、限制、审批、审计"的完整链路。

**本人做出的独立修正**：在 ChatGPT 建议的基础上，我增加了"敏感信息脱敏"（`redactSensitiveFields()`）作为审计日志的一部分，因为通用要求明确禁止凭据进入日志，而审计日志如果不脱敏就是合规漏洞。

---

## 三、AI 建议的采纳与推翻

### 采纳的 AI 建议（部分清单）

| 建议 | 来源 | 影响 |
|------|------|------|
| Agent Loop 引入 State + Stop Condition | ChatGPT 审阅 | 架构级修正 |
| Memory 按需检索 | ChatGPT 审阅 | 架构级修正 |
| Action 抽象层 | ChatGPT 审阅 | 架构级修正 |
| CREDENTIAL 模块 | ChatGPT 审阅 | 新增模块 |
| GovernanceAuth + ActionHash | ChatGPT 审阅 | 核心设计补充 |
| 审计日志 | ChatGPT 审阅 | 核心设计补充 |
| Tool base risk vs Effective Risk | ChatGPT 审阅 | 消除实现歧义 |
| Cold Start 验证 | ChatGPT 审阅 | 新增测试层 |
| WebUI 降级为 Thin Layer | ChatGPT 审阅 | 架构调整 |
| 部署安全边界 | ChatGPT 审阅 | 安全补充 |
| 每个 US 增加 Non-goals | ChatGPT 审阅 | 文档改进 |
| Credential 分阶段实现 | ChatGPT 审阅 | 工期管理 |
| MockProvider 函数模式 | ChatGPT 审阅 | 测试能力 |

### 推翻/修正的 AI 建议

| 建议 | 推翻理由 | 我的处理 |
|------|---------|---------|
| 建议删除 WebUI | 课程要求"线上可访问 WebUI 接口"，且 HITL 审批需要 WebUI | 保留 WebUI，但严格限制为 Thin Layer |
| 建议实现完整 Keychain 作为 MVP | 跨平台 Keychain 实现会占用大量工期，拖慢主线 | 分阶段：Phase 1 interface + mock + env + redact，Phase 2 Keychain |
| 建议治理深度用"三层就够了" | 课程要求"主角维度做深"，三层不够支撑"贡献"深度 | 升级为五层 + 审计日志，形成完整的可审计治理管线 |

---

## 四、反思：Brainstorming 技能的评估

### 做得好的方面

1. **追问质量高**：在定位、语言、深入维度等关键决策点，AI 的追问帮助我明确了之前模糊的假设。例如"你为什么选治理而非记忆"这个问题迫使我量化对比了两个维度的代码密度和可测试性。
2. **架构审阅的迭代深度**：从 V1 到 V2.2 经历了 4 轮审阅，每一轮都发现了新的问题——从"缺少 State"到"全量注入"到"Credential 缺失"到"Cold Start 缺失"。这说明多轮审阅比一轮通过更有效。
3. **边界控制**：AI 在多个节点提醒"不要继续无边界加功能"，这对项目范围控制很有价值。

### 不满意的方面

1. **审阅依赖第三方中转**：整个 brainstorming 过程中，ChatGPT 充当了"审阅者"角色，Claude Code 是"执行者"。这意味着我需要在一个 session 中（Claude Code）和另一个工具（ChatGPT）之间来回切换。理想情况下，同一工具应同时具备执行和深度审阅能力。
2. **审阅偶尔过于发散**：部分审阅轮次提出的建议过多（如第一轮 16 条意见），其中有些是确认性的而非修正性的。如果能更早区分"必须修正"和"建议性增强"，可以节省决策时间。
3. **冷启动验证的 agent 过于自主**：冷启动使用的 agent 在应该暂停确认的节点（如 Go 版本选择、make 安装）都自行决策了，导致部分缺陷未被及时捕获。这说明提示词"遇到不确定之处就暂停询问"的约束力有限，需要更明确的约束机制。

---

## 五、冷启动验证记录

### 5.1 基本信息

| 项目 | 描述 |
|------|------|
| 冷启动 agent 类型 | Claude Code（全新 session，无任何历史上下文或 memory） |
| 主开发 agent 类型 | Claude Code（当前 session，含完整 brainstorming 上下文） |
| 执行 task | P0.1（Go 模块初始化）+ P0.2（基础文件配置） |
| 提供材料 | 仅 SPEC.md + PLAN.md，不补充任何口头解释 |
| 约束指令 | "遇到不确定之处就暂停询问，不要凭猜测继续" |

### 5.2 暂停与提问记录

**事实：全程没有暂停/提问。**

冷启动 agent 在以下本应暂停的节点都自行决策了：

| 场景 | agent 的做法 | 本应做的 |
|------|-------------|---------|
| Go 未安装 | 自行用 winget 安装 Go 1.26.5 | 询问：SPEC 要求 Go 1.22，但 winget 安装的是 1.26.5，是否接受？ |
| make 未安装 | 自行用 winget 安装 make 4.4.1 | 询问：Windows 无 make，是否接受 winget 安装？或改用 go-task？ |
| 测试文件命名 | 创建了独立的测试文件 | 确认：P0.5 已经预留了骨架，测试应合入还是另建？ |

**结论**：冷启动 agent 过于"自主推进"，在应该暂停确认的节点都自行决策了。这本身就是一个过程反馈——提示词"遇到不确定之处就暂停询问"的约束力有限。

### 5.3 暴露的 SPEC 缺陷

#### 缺陷 1：Go 版本号不统一

| 位置 | 原表述 | 问题 |
|------|--------|------|
| §8.1 技术选型 | "Go"（无版本号） | 过于宽松 |
| §7.3.2 Dockerfile | `golang:1.22-alpine` | 锁定了 1.22 |
| §10.1 风险清单 | 未提及版本风险 | 未列出 |

**修订**：§8.1 明确为"Go 1.22+"，Dockerfile 锁定 `golang:1.22-alpine`，§10.1 增加"Go 版本环境差异"风险项。

#### 缺陷 2：项目根目录名称未定义

SPEC 附录的目录结构以 `harness/` 为根，但实际仓库名是 `CageHarness`。首次克隆的用户不知道目录应该叫什么。

**修订**：目录结构前增加注释"项目根目录名：`CageHarness/`（与 GitHub 仓库名一致）"。

#### 缺陷 3：构建产出位置未约定

| 位置 | 原表述 | 问题 |
|------|--------|------|
| §7.3.1 | `-o harness` | 产出在当前目录 |
| PLAN P0.1 | `make build` 产出二进制 | 未指定目录 |

**修订**：统一约定 `build/` 目录，§7.3.1 改为 `-o build/harness`，PLAN P0.1 相应更新。

#### 缺陷 4：make 可用性假设

SPEC 多处假设 `make` 可用（`make test`、`make build`），但 Windows 没有预装 make。这是隐式依赖。

**修订**：§10.1 增加"make 依赖"风险项，文档中写明 `make` 为可选依赖，提供等价 `go` 命令作为 fallback。

#### 缺陷 5：测试文件路径约定不完整

PLAN P0.5 预留了 `cold_start_test.go` 骨架，但 P0.1/P0.2 的验证步骤暗示需要独立测试，而 PLAN 没有明确说"把 P0.1/P0.2 的验证断言放进 P0.5 的骨架里"。

**修订**：PLAN P0.1/P0.2 的验证步骤中明确写明"测试归入 P0.5 骨架 `tests/demo/cold_start_test.go`，不另建独立测试文件"。

### 5.4 解读偏差

| 偏差 | 冷启动 agent 的解读 | 实际意图 | 责任方 |
|------|--------------------|---------|--------|
| P0.1/P0.2 测试归属 | 每个 task 各写一个测试文件 | 合入 P0.5 骨架 | 冷启动 agent 读错（PLAN 已明确 P0.5 用途） |
| 测试函数命名 | `TestModuleInit`、`TestGitignore` | `TestColdStart` + `t.Run()` 子测试 | 冷启动 agent 读错 |
| go.mod 的 go 版本 | 使用系统 1.26.5 | 1.22 | 冷启动 agent 擅自决定 |

### 5.5 产出与预期差距

| 维度 | 冷启动产出 | 预期 | 差距 |
|------|-----------|------|------|
| 测试文件 | 2 个独立测试文件，5 个独立测试函数 | 1 个 `cold_start_test.go`，`TestColdStart` + `t.Run()` | 大 — 文件结构和组织方式都不对 |
| Makefile 构建路径 | `-o build/harness` | 未明确，但冷启动的选择合理 | 无差距（已采纳为新的约定） |
| 功能满足度 | 验证了 .gitignore、config.example.yaml、.env.example | P0.1: 空测试套件可运行；P0.2: git init 验证 | 功能上满足，但组织方式偏离 |

### 5.6 对 SPEC/PLAN 的修订清单

| 修订 | 文件 | 变更内容 |
|------|------|---------|
| Go 版本统一 | SPEC §8.1 | 明确为"Go 1.22+" |
| 构建产出目录 | SPEC §7.3.1, PLAN P0.1 | 统一为 `build/` 目录 |
| 风险清单补充 | SPEC §10.1 | 增加 3 项风险：Go 版本、make 依赖、根目录名 |
| 根目录名 | SPEC 附录目录结构 | 增加注释"项目根目录名：CageHarness/" |
| 测试归属 | PLAN P0.1, P0.2 | 明确测试断言归入 P0.5 骨架 |
| 最小 main.go | PLAN P0.1 | 提前创建 `cmd/harness/main.go`（空实现） |

### 5.7 冷启动验证总结

**最有价值的 3 个发现**：

1. **SPEC 缺少"项目根目录命名约定"** — 最基础的信息缺失，新人首次搭建会困惑
2. **PLAN 的测试文件约定与早期 task 的验证需求之间存在张力** — P0.5 预留了骨架，但 P0.1/P0.2 的验证步骤暗示需要独立测试，而 PLAN 没有明确协调
3. **隐式依赖（make、Go 版本）没有在 SPEC 中纳入风险清单** — 在 Windows 环境首次运行会立即遇到这两个问题

**反思**：冷启动验证暴露的 5 个缺陷中，至少有 3 个（版本号、根目录名、测试归属）是"写了但没写清楚"的问题，而非"没写"的问题。这说明 SPEC 的精确性比完整性更难做到——可以在 SPEC 中写很多内容，但每一个细节是否足够精确到让一个全新 agent 无歧义地执行，是另一回事。这也正是课程要求冷启动验证的核心价值所在。

---

## 六、总体反思

### 6.1 Superpowers 方法论的评价

**发挥最大作用的技能**：`brainstorming` 和 `writing-plans` 的迭代式追问机制。如果没有多轮审阅，V1 架构会带着"全量注入"和"无 State 的流水线 Loop"进入实现，后者在实现阶段几乎必然导致重构。

**"形式大于实质"的部分**：TDD 的"红-绿-重构"流程在基础脚手架 task（如 P0.1 初始化）中略显形式化——为 `go.mod` 写测试的实际意义有限。但在核心机制 task（如 Governance 评估管线）中，TDD 的价值会充分体现。

### 6.2 SPEC/PLAN 质量对实现质量的影响

冷启动验证直接证明了：**SPEC 对基础脚手架 task 的描述足够清晰**（agent 没有因理解问题而暂停），但**PLAN 的 task 边界约定不够精确**（测试文件归属歧义、构建产出路径未约定）。

这印证了一个判断：对于本项目，SPEC 决定了"做什么"的清晰度，PLAN 决定了"怎么做"的精确度。两者质量缺一不可。

### 6.3 冷启动验证的价值评估

冷启动验证暴露的 5 个缺陷中，有 3 个（版本号、根目录名、测试归属）是"写了但没写精确"的问题。如果没有冷启动验证，这些问题会在实现阶段以"subagent 自行猜测并偏离预期"的形式暴露，届时修正成本更高。

**结论**：冷启动验证不是形式要求，而是单人项目中最接近"同侪评审"的内部机制，强烈建议保留在流程中。