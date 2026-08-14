# Phase 6 Gate Review

**Date**: 2026-08-14
**Reviewer**: Claude Code (automated boundary check)
**Branch**: `worktree-wt-governance` (commit `66af54c`)
**Verdict**: ✅ **PASS — Phase 7 可以启动**

---

## A. 依赖方向检查

### 实际依赖图（编译时）

```
             protocol (shared domain types)
               ↑
    ┌──────────┼──────────┐
    │          │          │
  agent   governance    tools

Domain Model + Independent Services 模式
三个包是兄弟关系，非调用链关系
```

### 运行时调用链（Phase 7 实现）

```
AgentRuntime (orchestrator，可能在 cmd/ 或 internal/runtime/)
  │
  │  同时 import agent, governance, tools, protocol
  │
  ├─→ llm.Generate(messages)          // LLM 思考
  ├─→ governance.Pipeline.Evaluate()  // 治理判断
  ├─→ tools.Registry.Execute()        // 工具执行
  └─→ agent.NewObservation()          // 观察记录
```

**关键区分**：编译时依赖 ≠ 运行时调用链。

- **编译时**：`agent`、`governance`、`tools` 三个包互相独立，仅共享 `protocol` 类型
- **运行时**：AgentRuntime 作为编排器，按顺序调用各模块

这是比最初设想的 `agent → governance → tools` 包依赖链更高级的设计：
- Agent 不强绑定治理实现
- Governance 可独立测试
- Tool 层可复用
- 运行时组装，而非编译时耦合

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
| A. 依赖方向（protocol ← agent/governance/tools，兄弟关系，无反向） | ✅ PASS |
| B. Action 类型唯一来源（protocol/action.go） | ✅ PASS |
| C. AgentState 唯一状态源（7 状态机） | ✅ PASS |
| D. Phase 7 验收标准明确 | ✅ 已定义 |
| E. 功能范围不膨胀 | ✅ 已约束 |
| F. 编译时依赖 ≠ 运行时调用链（Phase 7 不在 agent 包中 import governance） | ✅ 已明确 |

**Phase 6 Gate Review: ✅ 通过。可以进入 Phase 7 Agent Loop。**

**Phase 7 关键约束**：保持当前 package DAG，不为实现调用链而增加 `agent→governance` 或 `governance→tools` 的 import。运行时编排链在 AgentRuntime 层组装，而非包依赖层实现。