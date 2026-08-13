# Implementation Gate Review

> Phase 0 开始前的可执行性确认，不涉及任何架构变更。

---

## A. 仓库状态

| 检查项 | 状态 |
|--------|:---:|
| 当前 git 仓库 | ❌ 未初始化 → Phase 0 初始化 |
| 当前 branch | N/A |
| go.mod | ❌ 未初始化 → Phase 0 P0.1 初始化 |
| module path | `github.com/tyXiang-520/CageHarness`（已确认，与 GitHub 仓库名一致） |
| 工作区文件 | 仅含 SPEC.md、PLAN.md、SPEC_PROCESS.md 及课程要求文件 |

**结论**：状态正常，Phase 0 从零开始。

---

## B. Phase 0 边界

**Phase 0 只做 skeleton，不实现任何业务逻辑。**

| 允许 | 不允许 |
|------|--------|
| `go.mod` / `go.sum` | Governance 逻辑（Risk、Policy、HITL） |
| `cmd/harness/main.go`（空 `func main() {}`） | Agent 调度（Agent.Run、Loop） |
| `Makefile`（build/test/clean/run） | Memory 存储 |
| `.gitignore` / `config.example.yaml` / `.env.example` | Credential 存储 |
| `Dockerfile`（多阶段构建） | Tool 实现（FileTool、ShellTool） |
| `.gitlab-ci.yml` | Feedback Parser |
| `tests/demo/cold_start_test.go`（骨架，断言占位） | 治理审计日志 |
| 空目录结构：`internal/agent/`、`internal/llm/` 等 | 任何 Tool、Agent、Governance 的函数实现 |

**原则**：Phase 0 产出是一个"空但可构建"的框架。所有 `*.go` 文件要么是 `package main` 的空入口，要么是测试骨架。

---

## C. Package 依赖图（确认无循环）

```
cmd/harness/main.go
  └── 无依赖（空入口，Phase 11 才引入 internal 包）

internal/agent/          ← 依赖 llm, tools, governance, memory, config
  ├── action.go          ← Action 类型（无依赖）
  ├── state.go           ← AgentState（无依赖）
  ├── context.go         ← 依赖 llm, memory
  ├── loop.go            ← 依赖 llm, tools, governance, feedback, memory, config
  └── ...

internal/llm/            ← 无 internal 依赖
  ├── interface.go       ← 纯接口
  ├── mock.go
  └── openai.go

internal/tools/          ← 依赖 agent（action.go）
  ├── registry.go        ← 依赖 agent.Action
  ├── tool.go            ← Tool 接口
  └── file.go, shell.go, test.go

internal/governance/     ← 依赖 agent（action.go）
  ├── evaluator.go       ← 依赖 agent.Action
  ├── risk.go, policy.go, boundary.go, control.go
  ├── hitl.go
  ├── auth.go            ← 依赖 agent.Action
  └── audit.go

internal/feedback/       ← 无 internal 依赖
  ├── observation.go
  ├── test_parser.go
  └── shell_parser.go

internal/memory/         ← 无 internal 依赖
  ├── entry.go
  ├── store.go
  └── retriever.go

internal/runtime/        ← 依赖 agent
  └── task.go

internal/config/         ← 无 internal 依赖
  └── config.go

internal/credential/     ← 无 internal 依赖
  └── store.go, redact.go

web/                     ← 依赖 runtime, governance
  ├── server.go
  └── handler.go
```

**关键检查**：`agent.Action` 在 `internal/agent/action.go` 中定义，`tools` 和 `governance` 都依赖它。`agent` 不依赖 `tools` 或 `governance` 的具体实现（只依赖接口）。**无循环依赖风险。**

---

## D. 测试策略确认

| 测试类型 | 文件位置 | 说明 |
|---------|---------|------|
| 冷启动回归测试 | `tests/demo/cold_start_test.go` | **保留为回归测试**，每次环境变更后运行 |
| 治理拦截演示 | `internal/governance/evaluator_test.go` 追加 | 独立可运行 |
| 反馈闭环演示 | `internal/agent/loop_test.go` 追加 | 独立可运行 |
| 审计日志演示 | `internal/governance/audit_test.go` 追加 | 独立可运行 |
| 端到端测试 | `tests/e2e_test.go` | 集成所有模块 |

**确认**：`TestColdStart` 保留为回归测试，在每次 Phase 完成时运行，确保系统依然能从零引导。

---

## E. Worktree 合并策略

```
main (受保护)
  └── phase-branches (阶段性合并)
        └── feature-worktrees (subagent 工作区)
```

**流程**：
1. 每个 worktree 由 subagent 在独立分支上开发
2. 完成后创建 PR（Pull Request）到对应的 phase-branch
3. 人工评审（spec 合规 + 代码质量）后合并
4. Phase 完成时，phase-branch 合并回 main
5. 13 个 worktree 不直接合并到 main，避免冲突

**关键规则**：subagent 永远不直接 push 到 main。

---

## F. Definition of Done（每个 Phase/Worktree 完成标准）

| 检查项 | 命令 | 必须通过 |
|--------|------|:-------:|
| 单元测试 | `go test ./...` | ✅ |
| 代码检查 | `go vet ./...` | ✅ |
| 构建 | `go build ./...` | ✅ |
| 冷启动回归 | `go test -run TestColdStart ./tests/demo/...` | ✅（Phase 7 开始） |
| 代码格式 | `gofmt -d .` | 建议（非必须） |
| Lint | `golangci-lint` | ⏸ Phase 2 引入 |

---

## G. MockProvider 接口确认

确认采用**函数模式**，而非纯序列模式：

```go
// 正确的设计：可根据输入 messages 动态决策
type MockProvider struct {
    Handler func(messages []Message) (Response, error)
}

// 辅助：序列模式作为函数模式的特殊实现
func SequenceHandler(responses []ScriptedResponse) func(messages []Message) (Response, error) {
    callCount := 0
    return func(messages []Message) (Response, error) {
        if callCount >= len(responses) {
            return Response{}, ErrScriptExhausted
        }
        r := responses[callCount]
        callCount++
        return r, nil
    }
}
```

**关键**：`Handler` 可访问 `messages`，从而能够验证 Observation 是否注入下一轮 Context。纯序列模式无法做到这一点。

---

## H. Task Manager 职责边界

| 责任 | 不属于 |
|------|--------|
| 创建 Task（`Create(goal) → Task`） | Agent 决策逻辑 |
| 管理 Task 状态（Pending→Running→Done） | Governance 评估 |
| 启动 Agent goroutine | Tool 执行 |
| 提供状态查询（`Get(id) → Task`） | 反馈解析 |
| 支持取消（`Cancel(id)`） | 记忆写入 |
| 维护 Task 生命周期 | 凭据管理 |

**原则**：Task Manager 是编排层，不是业务逻辑层。它调用 Agent.Run()，但不实现 Agent.Run()。

---

## Gate Review 结论

| 检查项 | 状态 |
|--------|:---:|
| A. 仓库状态 | ✅ 可开始 |
| B. Phase 0 边界 | ✅ 已确认 |
| C. 依赖图无循环 | ✅ 已确认 |
| D. 测试策略 | ✅ Cold Start 保留为回归测试 |
| E. Worktree 合并策略 | ✅ 已确认 |
| F. Definition of Done | ✅ 已确认 |
| G. MockProvider 接口 | ✅ 函数模式 |
| H. Task Manager 边界 | ✅ 已确认 |

**Gate: PASS. 可以进入 Phase 0 实现。**