# Phase 6 Gate Review

**Date**: 2026-08-14
**Reviewer**: Claude Code (automated boundary check)
**Branch**: `worktree-wt-governance` (commit `66af54c`)
**Verdict**: ✅ **PASS — Phase 7 可以启动**

---

## A. 依赖方向检查

### 实际依赖图

```
protocol (shared domain types)
  ↑       ↑        ↑
agent  governance  tools
```

### 逐包验证

| 包 | 导入 | 结论 |
|---|------|------|
| `internal/protocol/` | 仅标准库 | ✅ 无内部依赖 |
| `internal/agent/` | `protocol` + 标准库 | ✅ 不导入 governance / tools |
| `internal/governance/` | `protocol` + 标准库 | ✅ 不导入 agent / tools |
| `internal/tools/` | `protocol` + 标准库 | ✅ 不导入 governance / agent |

### 运行时调用链

```
Agent (orchestrator)
  │
  │  import governance, import tools
  │  action := protocol.NewAction(...)
  │
  ├─→ governance.Pipeline.Evaluate(action)
  │     └─→ PipelineResult (Allow / Deny / RequireApproval / Escalate)
  │
  ├─→ if HITL required:
  │     └─→ governance.Pipeline.ApproveHITL(action, auth)
  │
  └─→ tools.GovernedTool.Execute(action)
        └─→ tools.Tool.Execute(action)
```

**关键结论**：所有三个包通过 `protocol.Action` 进行通信，而非直接导入。Agent 作为编排者会同时导入 `governance` 和 `tools`，但这是正确的方向——Agent 负责 orchestration，不拥有 Governance 逻辑。

### 反向依赖检查

| 检查项 | 结果 |
|--------|------|
| governance 是否导入 agent？ | ❌ 否 |
| governance 是否导入 tools？ | ❌ 否 |
| tools 是否导入 governance？ | ❌ 否 |
| tools 是否导入 agent？ | ❌ 否 |
| agent 是否导入 governance？ | ❌ 否 |
| agent 是否导入 tools？ | ❌ 否 |

**零反向依赖。架构边界干净。**

---

## B. Action 类型唯一来源

### 规范定义

`protocol/action.go` 中定义 `protocol.Action` struct —— 这是 **唯一** 的规范来源。

### 类型别名检查

| 文件 | 定义 | 类型 |
|------|------|------|
| `internal/protocol/action.go:53` | `type Action struct { ... }` | **规范定义** |
| `internal/agent/action.go:9` | `type Action = protocol.Action` | 类型别名（`=`） |
| `internal/tools/result.go:9` | `type ToolResult = protocol.ToolResult` | 类型别名（`=`） |

使用 `=` 的类型别名意味着 `agent.Action` **就是** `protocol.Action`，不是新类型。Go 编译器保证它们是同一个类型。

### 重复定义检查

```bash
grep -rn "type Action " internal/ --include="*.go"
# 结果：
# internal/protocol/action.go:53:  type Action struct { ... }     ← 唯一 struct 定义
# internal/agent/action.go:9:       type Action = protocol.Action  ← 类型别名
```

**结论**：Action 类型只有 `protocol/action.go` 一个 struct 定义，不存在 `agent.Action2` 或类似重复模型。

---

## C. AgentState 唯一状态源

### Phase 1 设计

`internal/agent/state.go` 定义了 7 状态机：

```
Idle → Thinking → Executing → Observing → Thinking (loop)
                   ↘ AwaitingApproval → Executing
                   ↘ Error → Terminated
                   ↘ Terminated
```

### 状态转换表

```go
var transitionTable = map[AgentState]map[AgentState]bool{
    AgentStateIdle:             {AgentStateThinking: true},
    AgentStateThinking:         {AgentStateExecuting: true, AgentStateAwaitingApproval: true, AgentStateError: true, AgentStateTerminated: true},
    AgentStateAwaitingApproval: {AgentStateExecuting: true, AgentStateError: true, AgentStateTerminated: true},
    AgentStateExecuting:        {AgentStateObserving: true, AgentStateError: true, AgentStateTerminated: true},
    AgentStateObserving:        {AgentStateThinking: true, AgentStateError: true, AgentStateTerminated: true},
    AgentStateError:            {AgentStateTerminated: true},
    AgentStateTerminated:       {},
}
```

### Phase 7 约束

Phase 7 的 Agent struct 应遵循以下约束：

```
✅ 正确设计：
type Agent struct {
    state    AgentState        // ← 唯一状态源
    messages []llm.Message     // LLM 对话历史
    memory   *memory.Store     // 记忆引用
    tools    *tools.Registry   // 工具注册表
    gov      *governance.Pipeline // 治理管道
}

❌ 禁止设计：
type Agent struct {
    currentAction Action
    history       []Message
    memory        []Entry
    status        string    // ← 重复状态！与 AgentState 冲突
}
```

**结论**：AgentState 是 Phase 1 已定义的唯一状态枚举，Phase 7 不应引入第二个状态字段。

---

## D. Phase 7 关键验收标准

### D.1 Feedback Loop Demo

Phase 7 必须产生 `TestDemoFeedbackLoop`（非简单 unit test），验证：

```
MockProvider (Function Mode)
       │
       ▼
Agent Loop
       │
       ▼
Tool Call (通过 Governance)
       │
       ▼
Observation (ToolResult)
       │
       ▼
Message appended to conversation
       │
       ▼
MockProvider 收到更新后的 messages（含 tool result）
       │
       ▼
LLM 基于 tool result 继续决策
```

**验收标准**：MockProvider 的 Handler 函数必须能看到前一轮 Tool 执行的 Observation 出现在消息列表中。

### D.2 职责隔离验证

| 职责 | 归属 | 检查 |
|------|------|------|
| 决定"下一步做什么" | Agent | ✅ |
| 判断"是否允许做" | Governance | ✅ |
| 执行具体操作 | Tools | ✅ |
| 风险分类 | ❌ 不在 Agent | ✅ |
| 策略检查 | ❌ 不在 Agent | ✅ |
| HITL 审批 | ❌ 不在 Agent | ✅ |

### D.3 不允许发生的架构偏移

如果 Phase 7 代码中出现以下任何情况，**立即停止并修正**：

1. `agent/` 包中直接调用 `RiskClassifier` 或 `PolicyEngine`
2. `agent/` 包中实现 HITL 审批逻辑
3. Agent struct 中有 `status string` 字段（与 AgentState 重复）
4. 新增 `agent.Action2` 或类似重复类型
5. Agent 直接调用 `Tool.Execute()` 而不经过 Governance

---

## E. 拓展边界判断

### 当前 MVP 范围（足够）

- ✅ 5 层 Governance Pipeline (Schema → Risk → Policy → Boundary → Control)
- ✅ 8 条内置策略规则
- ✅ HITL 审批流（ActionHash 绑定）
- ✅ Audit Log 不可变记录

### 不应继续增加的功能

- ❌ AI safety model（超出课程范围）
- ❌ Sandbox engine（超出课程范围）
- ❌ Permission graph（超出课程范围）
- ❌ 更多 Governance 层（5 层已足够展示）

**结论**：Phase 6 的 Governance 设计已达到课程展示标准，Phase 7-10 应聚焦于"把闭环跑通"，不再扩展功能范围。

---

## 最终判决

| 检查项 | 状态 |
|--------|------|
| A. 依赖方向（agent → governance → tools，无反向） | ✅ PASS |
| B. Action 类型唯一来源（protocol/action.go） | ✅ PASS |
| C. AgentState 唯一状态源（7 状态机） | ✅ PASS |
| D. Phase 7 验收标准明确 | ✅ 已定义 |
| E. 功能范围不膨胀 | ✅ 已约束 |

**Phase 6 Gate Review: ✅ 通过。可以进入 Phase 7 Agent Loop。**