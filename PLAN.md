# PLAN · Coding Agent Harness

> **Spec-Driven, Subagent-Built, Human-Owned.**
>
> 基于 SPEC V2.2 的实现计划
> 总任务数：61 个（含 5 个演示/验证任务）
> 预计实现周期：分 14 个 Phase（Phase 0–13），可并行 Phase 见依赖图

---

## 依赖总图

```
Phase 0: Scaffolding
    │
    ▼
Phase 1: Core Types  ←──────────────────────────────┐
    │               │                                │
    ├──▶ Phase 2: LLM Layer                          │
    ├──▶ Phase 3: Config                             │
    ├──▶ Phase 4: Credential                         │
    ├──▶ Phase 5: Tools ──▶ Phase 8: Feedback        │
    │                                  │              │
    ├──▶ Phase 6: Governance ★         │              │
    │               │                  │              │
    └──────┬────────┘                  │              │
           ▼                           ▼              │
    Phase 7: Agent Loop               │              │
           │                          │              │
           ├──────────────────────────┘              │
           ▼                                         │
    Phase 9: Memory (独立于 Agent Loop) ◄─────────────┘
           │
    Phase 10: Runtime
           │
    Phase 11: CLI ──▶ Phase 12: WebUI
           │
    Phase 13: Integration + Demo
```

**可并行 Phase**（使用独立 worktree）：
- Phase 2 (LLM)、Phase 3 (Config)、Phase 4 (Credential)、Phase 5 (Tools + Feedback)、Phase 6 (Governance)、Phase 9 (Memory) 彼此无依赖，可并行开发
- 但 Phase 7 (Agent Loop) 需等待 Phase 2、3、5、6 完成后才能开始
- Phase 10 (Runtime) 需等待 Phase 7
- Phase 11 (CLI) 需等待 Phase 7、4、9
- Phase 12 (WebUI) 需等待 Phase 10

---

## Phase 0：项目脚手架

> 无代码依赖，所有 task 可并行（同一 worktree 或独立 worktree）

### P0.1：Go 模块初始化

| 属性 | 值 |
|------|-----|
| **目标** | 初始化 Go module，搭建基础项目结构 |
| **涉及文件** | `go.mod`, `go.sum`, `Makefile`, `cmd/harness/main.go`（最小入口） |
| **实现要点** | `go mod init github.com/tyXiang-520/CageHarness`；Makefile 包含 `build`（`go build -o build/harness ./cmd/harness`）、`test`（`go test ./...`）、`clean`（`rm -rf build/`）、`run`（`go run ./cmd/harness`）目标；`cmd/harness/main.go` 仅含 `package main` + `func main() {}` 空实现 |
| **验证步骤** | `go test ./...` 可运行（空测试套件通过）；`make build` 产出 `build/harness` 二进制；`go vet ./...` 通过。**测试归入 P0.5 骨架 `tests/demo/cold_start_test.go`**，不另建独立测试文件 |
| **依赖** | 无 |

### P0.2：基础文件配置

| 属性 | 值 |
|------|-----|
| **目标** | 创建版本控制与配置模板 |
| **涉及文件** | `.gitignore`, `config.example.yaml`, `.env.example` |
| **实现要点** | .gitignore 覆盖 `.env`、`*.exe`、`build/`、`.harness/`、`harness` 二进制；config.example.yaml 包含所有配置项及其默认值/注释；.env.example 为 key 格式模板 |
| **验证步骤** | `git init` 后检查 `.env` 是否被 .gitignore 排除。**测试归入 P0.5 骨架 `tests/demo/cold_start_test.go`**，不另建独立测试文件 |
| **依赖** | P0.1 |

### P0.3：Docker 构建

| 属性 | 值 |
|------|-----|
| **目标** | 创建 Docker 多阶段构建配置 |
| **涉及文件** | `Dockerfile` |
| **实现要点** | 多阶段构建：golang:1.22-alpine 构建 → alpine:3.19 运行；非 root 用户；EXPOSE 8080；ENTRYPOINT ["harness"] |
| **验证步骤** | `docker build -t cageharness .` 成功；`docker run cageharness --help` 输出帮助信息 |
| **依赖** | P0.1 |

### P0.4：CI 配置

| 属性 | 值 |
|------|-----|
| **目标** | 创建 CI 配置文件 |
| **涉及文件** | `.gitlab-ci.yml` |
| **实现要点** | 必须包含名为 `unit-test` 的 job；运行 `make test`；可选：构建二进制并上传为 artifact |
| **验证步骤** | 提交后 CI 触发 unit-test job 并 pass |
| **依赖** | P0.1 |

### P0.5：Cold Start 测试骨架

| 属性 | 值 |
|------|-----|
| **目标** | 创建冷启动测试框架文件，结构先行 |
| **涉及文件** | `tests/demo/cold_start_test.go` |
| **实现要点** | 创建测试文件，包含：`TestColdStart` 函数签名 + 8 个断言的 `t.Run()` 子测试骨架（内容留空，随 Phase 推进逐步填充）；`NewTempEnvironment()` 辅助函数签名（创建临时 workspace）；`Bootstrap()` 辅助函数签名（初始化系统） |
| **验证步骤** | `go test -list . ./tests/demo/...` 列出 TestColdStart（测试体为空，不 assert 任何内容，仅验证编译通过） |
| **依赖** | P0.1 |

---

## Phase 1：核心类型定义

> 顺序执行（后一个 task 依赖前一个的类型定义）

### P1.1：Agent 状态类型

| 属性 | 值 |
|------|-----|
| **目标** | 定义 AgentState、AgentStatus、StopCondition 等核心状态类型 |
| **涉及文件** | `internal/agent/state.go`, `internal/agent/state_test.go` |
| **实现要点** | AgentStatus 枚举（idle/running/waiting_approval/done/failed/rejected）；StopCondition 枚举（5 种）；AgentState struct（Goal, Iteration, Messages, PendingAction, LastObservation, Status, StopCondition）；StopCondition→Status 映射表 |
| **验证步骤** | **TDD：先写测试** — 断言 Idle 状态默认值正确；断言每种 StopCondition 映射到正确的 Status；断言 Status 字符串化正确 |
| **依赖** | P0.1 |

### P1.2：Action 类型

| 属性 | 值 |
|------|-----|
| **目标** | 定义 Agent 与 Tools 之间的协议类型 |
| **涉及文件** | `internal/agent/action.go`, `internal/agent/action_test.go` |
| **实现要点** | Action struct（Tool string, Params map[string]any）；`canonicalJSON()` 函数（key 排序、递归规范化）；`Action.Hash()` 方法（SHA256(canonicalJSON)） |
| **验证步骤** | **TDD：先写测试** — 断言同一 Action 两次 Hash 一致；断言字段顺序不影响 Hash；断言不同 Action 产生不同 Hash |
| **依赖** | P1.1 |

### P1.3：Observation 类型

| 属性 | 值 |
|------|-----|
| **目标** | 定义统一的反馈观察类型 |
| **涉及文件** | `internal/feedback/observation.go`, `internal/feedback/observation_test.go` |
| **实现要点** | Observation struct（Success, Source, ErrorType, Details）；TestFailureDetail struct；ShellErrorDetail struct；String() 方法用于日志输出 |
| **验证步骤** | **TDD：先写测试** — 断言 Observation 构造、字段访问、字符串化正确 |
| **依赖** | P1.1 |

### P1.4：Governance 类型

| 属性 | 值 |
|------|-----|
| **目标** | 定义治理相关类型：DangerLevel、DecisionType、GovernanceDecision、GovernanceAuth |
| **涉及文件** | `internal/governance/decision.go`, `internal/governance/auth.go`, `internal/governance/decision_test.go`, `internal/governance/auth_test.go` |
| **实现要点** | DangerLevel 枚举（safe/suspicious/dangerous）；DecisionType 枚举（allow/reject/require_approval）；GovernanceDecision struct（Decision, Risk, Reasons, MatchedRules, Auth, ApprovalRequired）；GovernanceAuth struct（DecisionID UUID, ActionHash SHA256, ExpiresAt time.Time）；Auth 在 Allow 和 RequireApproval→Approve 两种情况下签发 |
| **验证步骤** | **TDD：先写测试** — 断言每种 DecisionType 正确；断言 Auth 构造与过期验证；断言 Auth.ActionHash 与 Action.Hash() 一致 |
| **依赖** | P1.2 |

### P1.5：Memory 条目类型

| 属性 | 值 |
|------|-----|
| **目标** | 定义记忆条目数据类型 |
| **涉及文件** | `internal/memory/entry.go`, `internal/memory/entry_test.go` |
| **实现要点** | MemoryEntry struct（ID, Type, Tags, Content, CreatedAt, AccessedAt）；NewMemoryEntry() 构造函数（自动生成 ID 和时间戳） |
| **验证步骤** | **TDD：先写测试** — 断言新条目 ID 不为空、CreatedAt 正确、Tags 存储正确 |
| **依赖** | P1.1 |

### P1.6：Config 类型

| 属性 | 值 |
|------|-----|
| **目标** | 定义完整的配置结构体 |
| **涉及文件** | `internal/config/config.go`, `internal/config/config_test.go` |
| **实现要点** | Config struct（LLM: Endpoint, Model, MaxTokens, Timeout；Agent: MaxIterations, RunTimeout；Governance: Enabled, HITLTimeout, ToolTimeout, WorkspaceRoot, Rules；Web: Port）；Duration 类型（支持 `yaml:"30s"` 解析）；DefaultConfig() 返回内置默认值 |
| **验证步骤** | **TDD：先写测试** — 断言 DefaultConfig() 各字段均不为零值；断言 Duration 解析 "30s" 正确 |
| **依赖** | P1.1 |

---

## Phase 2：LLM 抽象层

> 依赖 Phase 1 的 Message 类型；可与 Phase 3/4/5/6/9 并行

### P2.1：Provider 接口 + Message 类型

| 属性 | 值 |
|------|-----|
| **目标** | 定义 LLM Provider 统一接口和 Message 类型 |
| **涉及文件** | `internal/llm/interface.go`, `internal/llm/types.go`, `internal/llm/interface_test.go` |
| **实现要点** | Message struct（Role, Content）；Provider interface（Chat(ctx, []Message) (Response, error)）；Response struct（Content, FinishReason, Usage） |
| **验证步骤** | **TDD：先写测试** — 断言 Message 构造正确；断言接口可被 mock 实现 |
| **依赖** | P1.1 |

### P2.2：MockProvider（序列模式）

| 属性 | 值 |
|------|-----|
| **目标** | 实现预定义响应序列的 MockProvider |
| **涉及文件** | `internal/llm/mock.go`, `internal/llm/mock_test.go` |
| **实现要点** | MockProvider struct（script, callCount）；ResponseScript（Responses []ScriptedResponse）；ScriptedResponse（Content, Error）；按序返回响应，脚本耗尽时返回 ErrScriptExhausted |
| **验证步骤** | **TDD：先写测试** — 断言：按序返回 3 个响应；第 4 次调用返回 ErrScriptExhausted；注入 Error 时返回对应 error |
| **依赖** | P2.1 |

### P2.3：MockProvider（函数模式）

| 属性 | 值 |
|------|-----|
| **目标** | 实现可根据输入 messages 动态决定响应的 MockProvider |
| **涉及文件** | `internal/llm/mock.go`（追加）, `internal/llm/mock_test.go`（追加） |
| **实现要点** | MockFunc 类型（func(messages []Message) (Response, error)）；MockProviderFunc struct；与序列模式统一实现 Provider 接口 |
| **验证步骤** | **TDD：先写测试** — 断言：构造一个检查 messages 中是否包含特定关键字的 MockFunc，注入不同 messages 得到不同响应 |
| **依赖** | P2.1, P2.2 |

### P2.4：OpenAI Provider

| 属性 | 值 |
|------|-----|
| **目标** | 实现 OpenAI 兼容 API 的真实 Provider |
| **涉及文件** | `internal/llm/openai.go`, `internal/llm/openai_test.go` |
| **实现要点** | OpenAIProvider struct（endpoint, apiKey, model, httpClient）；Chat() 方法：构造 HTTP 请求 → 调用 Chat Completions API → 解析响应；支持配置超时；错误类型分类（auth_error, rate_limit, timeout, server_error, parse_error） |
| **验证步骤** | **TDD：先写测试** — 使用 httptest.NewServer mock API 端点，断言：正常响应被正确解析；400 返回 auth_error；429 返回 rate_limit；超时返回 timeout |
| **依赖** | P2.1 |

---

## Phase 3：配置加载

> 依赖 Phase 1 的 Config 类型；可与 Phase 2/4/5/6/9 并行

### P3.1：Config 加载与解析

| 属性 | 值 |
|------|-----|
| **目标** | 实现 YAML 配置文件的加载与解析 |
| **涉及文件** | `internal/config/config.go`（追加）, `internal/config/config_test.go`（追加）, `config.example.yaml` |
| **实现要点** | Load(path string) (*Config, error) 函数：读取 YAML 文件 → yaml.Unmarshal → 合并默认值 → 返回；内置 `slog` 日志输出加载的配置项（不输出 key） |
| **验证步骤** | **TDD：先写测试** — 构造临时 YAML 文件，断言 Load 正确解析所有字段；断言缺失字段使用默认值；断言文件不存在返回错误 |
| **依赖** | P1.6 |

### P3.2：Config 校验

| 属性 | 值 |
|------|-----|
| **目标** | 实现配置校验逻辑 |
| **涉及文件** | `internal/config/config.go`（追加）, `internal/config/config_test.go`（追加） |
| **实现要点** | Validate() 方法：检查必填项（LLM.Endpoint, LLM.Model）；检查范围（MaxIterations > 0, Port 在 1024-65535）；检查格式（WorkspaceRoot 路径是否存在）；返回明确的错误列表 |
| **验证步骤** | **TDD：先写测试** — 断言空 Endpoint 返回错误；断言 MaxIterations=0 返回错误；断言 Port=80 返回错误（非 root 无法绑定）；断言全部合法时返回 nil |
| **依赖** | P3.1 |

---

## Phase 4：凭据安全

> 依赖 Phase 1 的类型；可与 Phase 2/3/5/6/9 并行

### P4.1：CredentialStore 接口 + MockStore

| 属性 | 值 |
|------|-----|
| **目标** | 定义凭据存储接口和测试用 Mock 实现 |
| **涉及文件** | `internal/credential/store.go`, `internal/credential/store_test.go` |
| **实现要点** | CredentialStore interface（Set, Get, Delete, Exists）；MockStore（in-memory map 实现，供测试用）；ErrNotFound 错误类型 |
| **验证步骤** | **TDD：先写测试** — 断言 Set+Get 返回正确值；断言 Delete 后 Exists 返回 false；断言 Get 不存在的 key 返回 ErrNotFound |
| **依赖** | P1.1 |

### P4.2：EnvStore（环境变量 / .env 兼容）

| 属性 | 值 |
|------|-----|
| **目标** | 实现从环境变量和 .env 文件读取凭据的兼容输入源 |
| **涉及文件** | `internal/credential/env.go`, `internal/credential/env_test.go` |
| **实现要点** | EnvStore struct：从环境变量读取（`os.Getenv`）；支持从 `.env` 文件加载（`os.ReadFile` + 简单 key=value 解析）；Set 操作仅设置进程环境变量，不写回文件；文档说明明文风险 |
| **验证步骤** | **TDD：先写测试** — 设置环境变量后断言 Get 正确；写入 .env 文件后断言加载正确；断言不存在的 key 返回 ErrNotFound |
| **依赖** | P4.1 |

### P4.3：敏感信息脱敏

| 属性 | 值 |
|------|-----|
| **目标** | 实现审计日志和日志输出中的凭据字段脱敏 |
| **涉及文件** | `internal/credential/redact.go`, `internal/credential/redact_test.go` |
| **实现要点** | RedactSensitiveFields(params map[string]any) map[string]any；敏感字段列表：token, password, api_key, authorization, secret, key, credential；替换为 `"***REDACTED***"`；递归处理嵌套 map |
| **验证步骤** | **TDD：先写测试** — 断言 `{"command": "curl -H 'Authorization: ...'"}` 中的 Authorization 字段被脱敏；断言嵌套 map 递归脱敏；断言不含敏感字段时原样返回 |
| **依赖** | P4.1 |

### P4.4：KeychainStore（Phase 2，延后）

| 属性 | 值 |
|------|-----|
| **目标** | 实现 OS Keychain 安全存储 |
| **涉及文件** | `internal/credential/keychain.go`, `internal/credential/keychain_test.go` |
| **实现要点** | macOS：`security` 命令行；Windows：`powershell` Credential Manager 命令；Linux：`secret-tool`；所有平台统一实现 CredentialStore 接口 |
| **验证步骤** | 集成测试（仅在相应平台上运行，通过 build tags 控制） |
| **依赖** | P4.1 |
| **状态** | ⏸ **Phase 2（延后）** — MVP 先使用 EnvStore，Phase 2 实现后替换 |

---

## Phase 5：工具系统

> 依赖 Phase 1 的 Action 类型；可与 Phase 2/3/4/6/9 并行

### P5.1：Tool 接口 + ToolRegistry

| 属性 | 值 |
|------|-----|
| **目标** | 定义 Tool 接口和注册表的核心逻辑 |
| **涉及文件** | `internal/tools/tool.go`, `internal/tools/registry.go`, `internal/tools/registry_test.go` |
| **实现要点** | Tool interface（Name, Description, Parameters, Execute）；ToolRegistry struct（tools map, Validate, ExecuteApproved）；Validate(action) 校验工具是否存在、参数是否符合 schema；ExecuteApproved(ctx, action, auth) 校验 auth.ActionHash 与 action 一致、auth 未过期，然后调用 tool.Execute() |
| **验证步骤** | **TDD：先写测试** — 断言注册工具后 Find 返回正确；断言未注册工具 Validate 返回错误；断言 ExecuteApproved 校验 ActionHash 不一致时拒绝；断言过期 auth 被拒绝 |
| **依赖** | P1.2, P1.4 |

### P5.2：FileTool（read_file, write_file, list_files）

| 属性 | 值 |
|------|-----|
| **目标** | 实现文件操作工具 |
| **涉及文件** | `internal/tools/file.go`, `internal/tools/file_test.go` |
| **实现要点** | 三个工具函数：read_file（`os.ReadFile`）、write_file（`os.WriteFile`）、list_files（`os.ReadDir`）；所有操作限制在 workspace root 内（前置检查）；路径规范化（`filepath.Clean`）后检查是否越界 |
| **验证步骤** | **TDD：先写测试** — 在临时目录中：断言 read_file 读取正确；断言 write_file 写入正确；断言 list_files 列出正确；断言 `../` 越界操作返回 ErrPathEscape |
| **依赖** | P5.1 |

### P5.3：ShellTool（execute_shell）

| 属性 | 值 |
|------|-----|
| **目标** | 实现 shell 命令执行工具 |
| **涉及文件** | `internal/tools/shell.go`, `internal/tools/shell_test.go` |
| **实现要点** | ShellTool.Execute()：限制工作目录为 workspace root；使用 `context.WithTimeout(ctx, toolTimeout)` 强制执行超时；捕获 stdout/stderr/exit_code；最大输出长度限制（防止无限输出） |
| **验证步骤** | **TDD：先写测试** — 断言 `echo hello` 返回 stdout="hello"；断言 `exit 1` 返回 exit_code=1；断言超时后返回 ErrToolTimeout；断言工作目录限制生效 |
| **依赖** | P5.1 |

### P5.4：TestTool（run_tests）

| 属性 | 值 |
|------|-----|
| **目标** | 实现测试运行工具 |
| **涉及文件** | `internal/tools/test.go`, `internal/tools/test_test.go` |
| **实现要点** | TestTool.Execute()：执行 `go test -json -v ./...`（或指定 pattern）；捕获 stdout + stderr；解析退出码；返回结构化结果（pass/fail + 原始输出） |
| **验证步骤** | **TDD：先写测试** — 在含测试的临时目录中：断言 passing 测试返回 Success=true；断言 failing 测试返回 Success=false |
| **依赖** | P5.1 |

---

## Phase 6：治理系统（★ 深入维度）

> 依赖 Phase 1 的 Action/Governance 类型；可与 Phase 2/3/4/5/9 并行
> **这是项目的深入维度，测试最密集**

### P6.1：Risk Classification

| 属性 | 值 |
|------|-----|
| **目标** | 实现风险分类逻辑 |
| **涉及文件** | `internal/governance/risk.go`, `internal/governance/risk_test.go` |
| **实现要点** | Classify(action Action) DangerLevel：根据 Tool 的 base DangerLevel + Action 内容综合计算 EffectiveRisk。`execute_shell "go test ./..."` → Suspicious；`execute_shell "rm -rf /"` → Dangerous；`write_file "notes.txt"` → Suspicious；`read_file "notes.txt"` → Safe |
| **验证步骤** | **TDD：先写测试** — 构造 10 个不同 Action，断言每个的 EffectiveRisk 符合预期 |
| **依赖** | P1.2, P1.4 |

### P6.2：Policy Engine + 内置规则

| 属性 | 值 |
|------|-----|
| **目标** | 实现策略规则引擎和 5 条 MVP 内置规则 |
| **涉及文件** | `internal/governance/policy.go`, `internal/governance/policy_test.go` |
| **实现要点** | Rule interface（ID, Description, Match(action) → (matched bool, reasons []string)）；PolicyEngine struct（rules []Rule, Evaluate(action) → []RuleMatch）；内置规则：GIT-001 (push --force)、GIT-002 (reset --hard)、GIT-003 (clean -fd)、SHELL-001 (rm -rf /)、SHELL-002 (chmod 777)、FILE-001 (write to .git/)、NET-001 (curl/wget/nc)、PATH-001 (path escape)；规则匹配支持通配符/正则 |
| **验证步骤** | **TDD：先写测试** — 为每条规则构造匹配/不匹配的 Action，断言 Evaluate 正确返回匹配规则和不匹配规则 |
| **依赖** | P1.2, P1.4, P6.1 |

### P6.3：Execution Boundary

| 属性 | 值 |
|------|-----|
| **目标** | 实现路径边界、环境白名单、网络策略检查 |
| **涉及文件** | `internal/governance/boundary.go`, `internal/governance/boundary_test.go` |
| **实现要点** | CheckPath(action, workspaceRoot) error：规范化路径并检查是否在 workspace root 内；CheckEnv(action) []string：检查环境变量白名单，标记可疑的环境变量访问；CheckNetwork(action) (bool, string)：检查命令是否包含外发网络操作 |
| **验证步骤** | **TDD：先写测试** — 断言 `../` 逃逸被拦截；断言绝对路径 `/etc/passwd` 被拦截；断言 `curl` 命令被标记为网络操作；断言合法路径通过 |
| **依赖** | P1.2, P1.4 |

### P6.4：Execution Control

| 属性 | 值 |
|------|-----|
| **目标** | 实现执行控制策略检查 |
| **涉及文件** | `internal/governance/control.go`, `internal/governance/control_test.go` |
| **实现要点** | CheckTimeout(action, config) (allowed bool, suggestedTimeout time.Duration)：检查 Action 允许的最大执行时间；ValidateCancellation(action) bool：检查 Action 是否可安全取消。**注意**：真正的 timeout 强制在 `Tool.Execute()` 中使用 `context.WithTimeout()` 实现，Governance 层仅做策略检查 |
| **验证步骤** | **TDD：先写测试** — 断言 `run_tests` 返回合理的 suggestedTimeout；断言 `read_file` 的超时策略合理；断言 `execute_shell` 的超时策略合理 |
| **依赖** | P1.2, P1.4 |

### P6.5：HITL 状态机

| 属性 | 值 |
|------|-----|
| **目标** | 实现 HITL 状态机：Pending → Approved/Rejected/MoreInfo → Executing → Done |
| **涉及文件** | `internal/governance/hitl.go`, `internal/governance/hitl_test.go` |
| **实现要点** | HITLStateMachine struct：持有当前状态 (Pending/Approved/Rejected/MoreInfo/Executing/Done)、审批超时设置、pendingAction 和 pendingDecision；Approve() 方法：校验状态为 Pending → 生成 GovernanceAuth → 转换到 Executing；Reject() 方法：转换到 Rejected；MoreInfo(feedback string) 方法：记录 humanFeedback，转换到 MoreInfo 状态；CheckTimeout() 方法：超时自动 Rejected；同一 Action 被 Rejected 后不可绕过审批再次提交 |
| **验证步骤** | **TDD：先写测试** — 断言：正常 Approve 流程正确生成 Auth；正常 Reject 后状态为 Rejected；Rejected 后同一 Action 再次提交被拒绝；MoreInfo 后携带 humanFeedback 可重新进入评估；超时自动 Rejected |
| **依赖** | P1.4, P6.1, P6.2 |

### P6.6：GovernanceAuth

| 属性 | 值 |
|------|-----|
| **目标** | 实现 Auth 签发与校验 |
| **涉及文件** | `internal/governance/auth.go`（追加）, `internal/governance/auth_test.go`（追加） |
| **实现要点** | NewAuth(action Action) *GovernanceAuth：生成 UUID、计算 ActionHash、设置 ExpiresAt；Validate(auth, action Action) error：校验 ActionHash 一致、未过期；Auth 签名不对应任何密码学签名（教学型 MVP，无需私钥） |
| **验证步骤** | **TDD：先写测试** — 断言 NewAuth 生成合法 Auth；断言不同 Action 产生的 Auth 校验时失败；断言过期 Auth 校验时返回 ErrAuthExpired |
| **依赖** | P1.2, P1.4 |

### P6.7：Evaluator（评估管线编排）

| 属性 | 值 |
|------|-----|
| **目标** | 实现完整的五层评估管线编排 |
| **涉及文件** | `internal/governance/evaluator.go`, `internal/governance/evaluator_test.go` |
| **实现要点** | Evaluator struct：持有 RiskClassifier、PolicyEngine、BoundaryChecker、ControlChecker、HITL；Evaluate(action) GovernanceDecision 方法按序执行：Schema Validation → Risk Classification → Policy Engine → Execution Boundary → Execution Control → Decision（Allow/Reject/RequireApproval）；所有决策结果通过 AuditRecorder 记录 |
| **验证步骤** | **TDD：先写测试** — 构造不同类型的 Action（安全/危险/越界），断言完整评估管线正确输出 Decision；断言每个决策的 Reasons 和 MatchedRules 正确填充 |
| **依赖** | P6.1, P6.2, P6.3, P6.4, P6.5, P6.6 |

### P6.8：Audit Log

| 属性 | 值 |
|------|-----|
| **目标** | 实现审计日志写入与读取 |
| **涉及文件** | `internal/governance/audit.go`, `internal/governance/audit_test.go` |
| **实现要点** | AuditRecorder struct：写入 `.harness/audit.jsonl`（JSONL 格式，每行一条 JSON）；AuditLogEntry struct（Timestamp, Action, Risk, Decision, MatchedRules, Reasons, HumanDecision, HumanReason）；写入前调用 `redactSensitiveFields()` 脱敏；Read() 方法读取完整日志（供 WebUI 展示）；自动创建 `.harness/` 目录 |
| **验证步骤** | **TDD：先写测试** — 断言写入后文件存在；断言 JSONL 格式正确（每行一个 JSON 对象）；断言脱敏生效（含敏感字段的 params 被自动替换）；断言 Read 返回正确的条目列表 |
| **依赖** | P4.3, P1.4 |

---

## Phase 7：Agent 主循环

> 依赖 Phase 2 (LLM)、Phase 3 (Config)、Phase 5 (Tools)、Phase 6 (Governance)
> 在所有依赖完成后才能开始

### P7.1：Context Assembly

| 属性 | 值 |
|------|-----|
| **目标** | 实现上下文组装器：将 system prompt、messages、memory、observation 组装为 LLM 请求 |
| **涉及文件** | `internal/agent/context.go`, `internal/agent/context_test.go` |
| **实现要点** | ContextAssembler struct：Assemble(state AgentState) []Message：system prompt（含工具描述、Governance 规则摘要、项目约定）+ user messages（含 Goal）+ memory 条目（按需检索）+ last observation（若有）；消息截断策略：超出 token 限制时从早期截断（保留最新的 N 条） |
| **验证步骤** | **TDD：先写测试** — 断言初始状态组装包含 system prompt + Goal；断言包含 LastObservation 时正确注入；断言超过截断限制后早期消息被移除 |
| **依赖** | P2.1, P3.1, P9.2 |

### P7.2：Agent Loop 状态机

| 属性 | 值 |
|------|-----|
| **目标** | 实现 Agent 主循环状态机 |
| **涉及文件** | `internal/agent/loop.go`, `internal/agent/loop_test.go` |
| **实现要点** | Agent struct（state, provider, config, toolRegistry, evaluator, memory, contextAssembler）；Run(ctx) 方法：状态机循环（Context Assembly → LLM → Parse Action → Governance → Tool Dispatch → Observation → State Update → Stop Check）；每次迭代输出结构化日志（slog）；异常处理：LLM 调用失败可重试（最多 2 次），工具执行失败不停止循环 |
| **验证步骤** | **TDD：先写测试（集成测试）** — 使用 MockProvider（序列模式），配置工具和 Governance，断言：Agent 从 Idle → Running → Done 状态转换正确；断言每轮迭代的日志输出包含 iteration 和 action |
| **依赖** | P7.1, P2.2, P5.1, P6.7 |

### P7.3：Stop Condition 检查

| 属性 | 值 |
|------|-----|
| **目标** | 实现 5 种停机条件的检查逻辑 |
| **涉及文件** | `internal/agent/loop.go`（追加）, `internal/agent/state.go`（追加）, `internal/agent/loop_test.go`（追加） |
| **实现要点** | CheckStop(state AgentState, config) (shouldStop bool, condition StopCondition)：检查顺序：Completed（Goal 达成）→ HumanRejected（HITL 拒绝）→ UnrecoverableError（LLM 不可恢复错误）→ Timeout（超过 run-timeout）→ MaxIterations（超过 max-iterations）；优先级如上 |
| **验证步骤** | **TDD：先写测试（集成测试）** — 使用 MockProvider 构造：达到 MaxIterations 后 StopCondition=MaxIterations；HITL 拒绝后 StopCondition=HumanRejected；超时后 StopCondition=Timeout |
| **依赖** | P7.2 |

### P7.4：Agent.Run() 完整生命周期

| 属性 | 值 |
|------|-----|
| **目标** | 实现完整的 Agent.Run() 方法，串联所有组件 |
| **涉及文件** | `internal/agent/loop.go`（追加）, `internal/agent/loop_test.go`（追加） |
| **实现要点** | Agent.Run(ctx, goal string) (result RunResult, err error)：RunResult struct（FinalStatus, StopCondition, TotalIterations, LastAction, FinalObservation）；完整生命周期：初始化状态 → 循环 → 返回结果；退出码映射：Done=0, Failed=1, Rejected=2 |
| **验证步骤** | **TDD：先写测试（集成测试）** — 使用 MockProvider 完整运行 3 轮迭代，断言 RunResult 的字段正确 |
| **依赖** | P7.3 |

### P7.5：Cold Start 验证（核心模块）

| 属性 | 值 |
|------|-----|
| **目标** | 填充冷启动测试中 5 个核心模块的断言，验证系统在空环境下能否完成一次完整闭环 |
| **涉及文件** | `tests/demo/cold_start_test.go`（填充） |
| **实现要点** | 在 P0.5 创建的骨架中填充以下断言（使用 MockProvider）：<br>**(1) Config 冷启动**：无 config 文件时，`LoadConfig()` 返回默认配置（非 nil、默认 model、默认 workspace root）<br>**(2) Credential 冷启动**：无 API key 时，使用 MockProvider fallback，不 crash<br>**(3) Tool Registry 冷启动**：`NewToolRegistry()` 包含全部 5 个 MVP 工具（read_file, write_file, list_files, execute_shell, run_tests）<br>**(4) Governance 冷启动**：对安全 Action（如 `read_file`）评估后 Decision 不为 nil，Auth 正确签发；对危险 Action 评估后返回 RequireApproval<br>**(5) Agent Loop 冷启动**：`Agent.Run()` 在 MockProvider 下完成一次完整闭环（Idle → Running → Done），产生 ToolResult 和 Observation<br>**(6) Memory 冷启动**：空 Store 初始化后 `Retrieve("")` 返回空切片（非 nil） |
| **验证步骤** | `go test -run TestColdStart ./tests/demo/... -v` 通过前 6 个子测试 |
| **依赖** | P7.4, P8.3, P9.1 |

---

## Phase 8：反馈闭环

> 依赖 Phase 5 (Tools) 的工具执行结果

### P8.1：TestParser

| 属性 | 值 |
|------|-----|
| **目标** | 实现 `go test -json` 输出的解析器 |
| **涉及文件** | `internal/feedback/test_parser.go`, `internal/feedback/test_parser_test.go` |
| **实现要点** | ParseTestOutput(stdout, stderr string, exitCode int) Observation：逐行解析 JSON 输出；提取测试结果（pass/fail）、失败测试名称和错误信息；生成 Observation{Success, Source:"go_test", ErrorType, Details:[]TestFailureDetail} |
| **验证步骤** | **TDD：先写测试** — 构造 `go test -json` 的示例输出（pass 和 fail 场景），断言：pass 时 Success=true；fail 时 Success=false, ErrorType="test_failure"；fail 时 Details 包含正确的 TestName 和 Message |
| **依赖** | P1.3 |

### P8.2：ShellParser

| 属性 | 值 |
|------|-----|
| **目标** | 实现 shell 执行结果的解析器 |
| **涉及文件** | `internal/feedback/shell_parser.go`, `internal/feedback/shell_parser_test.go` |
| **实现要点** | ParseShellResult(stdout, stderr string, exitCode int) Observation：exitCode=0 且 stderr 为空 → Success=true；exitCode≠0 或 stderr 非空 → Success=false, ErrorType="shell_error"；Details 包含 ExitCode 和 Stderr 摘要 |
| **验证步骤** | **TDD：先写测试** — 断言 exitCode=0 时 Success=true；断言 exitCode=1 时 Success=false；断言 stderr 非空时 ErrorType="shell_error" |
| **依赖** | P1.3 |

### P8.3：Feedback 集成到 Agent Loop

| 属性 | 值 |
|------|-----|
| **目标** | 将 Observation 回灌到 Agent State，在下一轮 Context Assembly 中注入 |
| **涉及文件** | `internal/agent/loop.go`（追加）, `internal/agent/context.go`（追加）, `internal/feedback/feedback_test.go` |
| **实现要点** | FeedbackProcessor struct：根据 ToolResult 类型选择对应的 Parser；FeedBack(agentState, toolResult) Observation：执行 → 更新 agentState.LastObservation → 返回 Observation；在 Agent Loop 中：Tool 执行后调用 FeedBack，将 Observation 存入 State |
| **验证步骤** | **TDD：先写测试（集成测试）** — 使用 MockProvider（函数模式），断言：注入 `Observation{Success:false}` 后，下一轮 Context 中包含该 Observation；same initial state + different Observation → Agent 不同 |
| **依赖** | P7.2, P8.1, P8.2 |

---

## Phase 9：记忆系统

> 无 Agent Loop 依赖，可与 Phase 2/3/4/5/6 并行

### P9.1：Memory Store（JSON 文件）

| 属性 | 值 |
|------|-----|
| **目标** | 实现 JSON 文件持久化的记忆存储 |
| **涉及文件** | `internal/memory/store.go`, `internal/memory/store_test.go` |
| **实现要点** | FileStore struct（path string）；Save(entry MemoryEntry) error：追加到 JSON 数组（先读全部 → 追加 → 写回）；Load() ([]MemoryEntry, error)：读取全部条目；Get(id string) (MemoryEntry, bool)：按 ID 查找；Delete(id string) error：删除指定条目；文件存储在 `.harness/memory.json` |
| **验证步骤** | **TDD：先写测试** — 断言 Save 后 Load 返回包含该条目；断言 Delete 后条目不存在；断言 Get 不存在的 ID 返回 false；断言文件格式正确（合法的 JSON 数组） |
| **依赖** | P1.5 |

### P9.2：Memory Retriever（标签匹配）

| 属性 | 值 |
|------|-----|
| **目标** | 实现基于关键词/标签的记忆检索器 |
| **涉及文件** | `internal/memory/retriever.go`, `internal/memory/retriever_test.go` |
| **实现要点** | Retriever struct（store）；Retrieve(taskContext string, limit int) []MemoryEntry：将任务描述分词；与条目的 Tags 和 Content 进行子串匹配；按匹配分数排序取 top-N；不匹配时返回空切片（非全量加载）；更新 AccessedAt |
| **验证步骤** | **TDD：先写测试** — 断言：明确匹配的条目被检索到；不匹配的条目不被检索（非全量加载）；空任务上下文返回空切片 |
| **依赖** | P9.1 |

### P9.3：Memory CLI 命令

| 属性 | 值 |
|------|-----|
| **目标** | 实现 `harness memory add/list` 命令 |
| **涉及文件** | `cmd/harness/memory.go`, `internal/memory/cli.go` |
| **实现要点** | memory add 命令：`--type`, `--tags`, `--content` 参数 → 创建 MemoryEntry → Save；memory list 命令：显示所有条目（type, tags, content 摘要（前 80 字符）, 创建时间） |
| **验证步骤** | 手动测试：`harness memory add --type=convention --tags=go,naming --content="Use snake_case"` 后 `harness memory list` 显示该条目 |
| **依赖** | P9.1 |

---

## Phase 10：运行时（Task Manager）

> 依赖 Phase 7 (Agent Loop)

### P10.1：Task Manager

| 属性 | 值 |
|------|-----|
| **目标** | 实现异步任务管理器，支持 WebUI 并发执行 |
| **涉及文件** | `internal/runtime/task.go`, `internal/runtime/task_test.go` |
| **实现要点** | Task struct（ID, Status, AgentState, CreatedAt, UpdatedAt, Result）；TaskManager struct：Create(goal string) Task（启动 goroutine 执行 Agent.Run()，立即返回 Task）；Get(id string) (Task, bool)（查询状态快照）；List() []Task（列出所有任务）；goroutine 中：Agent.Run() → 完成后更新 Task.Status 和 Task.Result |
| **验证步骤** | **TDD：先写测试** — 使用 MockProvider，断言：Create 后立即返回（不阻塞）；Get 返回正确的状态；Agent 完成后 Status 正确更新 |
| **依赖** | P7.4 |

### P10.2：Cold Start 验证（Runtime 补充）

| 属性 | 值 |
|------|-----|
| **目标** | 在冷启动测试中补充 Runtime Task Manager 和 Audit 的断言 |
| **涉及文件** | `tests/demo/cold_start_test.go`（追加） |
| **实现要点** | 在第 P7.5 已经填充的 6 个断言基础上追加：<br>**(7) Runtime Task Manager 冷启动**：TaskManager.Create() → 状态从 Pending → Running → Done 完整流转<br>**(8) Audit 冷启动**：Agent 执行一次完整任务后，audit.jsonl 包含至少一条记录，记录包含 Timestamp、Action、Decision |
| **验证步骤** | `go test -run TestColdStart ./tests/demo/... -v` 全部 8 个子测试通过 |
| **依赖** | P10.1, P6.8 |

---

## Phase 11：CLI 入口

> 依赖 Phase 7 (Agent Loop)、Phase 4 (Credential)、Phase 9 (Memory)

### P11.1：CLI 主入口

| 属性 | 值 |
|------|-----|
| **目标** | 实现 CLI 主入口和命令路由 |
| **涉及文件** | `cmd/harness/main.go` |
| **实现要点** | 使用标准库 `flag` 解析顶层命令；路由到 run/serve/init/memory/credential 子命令；全局 `--help` 输出帮助信息；启动时加载 Config，失败时输出友好错误 |
| **验证步骤** | `go run ./cmd/harness --help` 输出 5 个顶层命令列表 |
| **依赖** | P3.1 |

### P11.2：`harness run` 命令

| 属性 | 值 |
|------|-----|
| **目标** | 实现 `harness run --task "...'"` 命令 |
| **涉及文件** | `cmd/harness/run.go` |
| **实现要点** | 解析 `--task`, `--max-iterations`, `--run-timeout` 参数；从 CredentialStore 读取 API key；构造 Agent 实例；调用 Agent.Run()；输出：每次迭代显示 Iteration #、Action 类型、Tool 执行摘要、Observation 摘要；最终输出 FinalStatus、StopCondition、总迭代次数；退出码映射：Done=0, Failed=1, Rejected=2 |
| **验证步骤** | 使用 MockProvider 运行 `harness run --task "test"`，断言终端输出包含迭代信息 |
| **依赖** | P7.4, P4.1, P11.1 |

### P11.3：`harness run` 交互式 HITL

| 属性 | 值 |
|------|-----|
| **目标** | 实现 CLI 模式下的 HITL 审批交互 |
| **涉及文件** | `cmd/harness/run.go`（追加） |
| **实现要点** | 当 Governance 触发 RequireApproval 时：终端暂停 → 显示危险动作详情（Risk、规则、原因）→ 显示 `[A]pprove / [R]eject / [M]ore info` 提示 → 读取 stdin 输入 → 提交到 HITL 状态机；超时自动拒绝；输入验证（无效输入重新提示） |
| **验证步骤** | 手动测试：构造一个触发 GIT-002 规则的任务，断言终端显示审批提示并接受输入 |
| **依赖** | P11.2, P6.5 |

### P11.4：`harness serve` 命令

| 属性 | 值 |
|------|-----|
| **目标** | 实现 WebUI 服务启动命令 |
| **涉及文件** | `cmd/harness/serve.go` |
| **实现要点** | 解析 `--port` 参数（默认 8080）；从 CredentialStore 读取 API key；构造 TaskManager + WebUI Server；启动 HTTP 服务；输出 `Server started at http://localhost:8080` |
| **验证步骤** | `harness serve --port 8090` 启动后 `curl http://localhost:8090` 返回 200 |
| **依赖** | P10.1, P12.1, P11.1 |

### P11.5：`harness init` 命令

| 属性 | 值 |
|------|-----|
| **目标** | 实现首次运行的引导式初始化 |
| **涉及文件** | `cmd/harness/init.go` |
| **实现要点** | 检查 Config 是否存在（不存在则创建 `config.yaml` 模板）；检查 API key 是否已配置（未配置则隐藏输入引导录入，存入 CredentialStore）；创建 `.harness/` 目录；输出初始化结果 |
| **验证步骤** | 在空目录中运行 `harness init`，断言 `config.yaml` 和 `.harness/` 被创建 |
| **依赖** | P4.1, P3.1, P11.1 |

### P11.6：`harness credential` 命令

| 属性 | 值 |
|------|-----|
| **目标** | 实现凭据管理命令 |
| **涉及文件** | `cmd/harness/credential.go` |
| **实现要点** | `credential status`：显示所有已配置的 key 名称（不显示值）；`credential set --key=OPENAI_API_KEY`：隐藏输入 → 存入 CredentialStore；`credential delete --key=OPENAI_API_KEY`：确认后删除 |
| **验证步骤** | 手动测试：`harness credential set --key=OPENAI_API_KEY` 输入后 `harness credential status` 显示已配置 |
| **依赖** | P4.1, P11.1 |

### P11.7：`harness memory` 命令

| 属性 | 值 |
|------|-----|
| **目标** | 实现记忆管理命令 |
| **涉及文件** | `cmd/harness/memory.go` |
| **实现要点** | `memory add --type --tags --content`：创建并保存记忆条目；`memory list`：显示所有条目 |
| **验证步骤** | 手动测试：`harness memory add --type=convention --tags=test --content="test"` 后 `harness memory list` 显示该条目 |
| **依赖** | P9.3, P11.1 |

---

## Phase 12：WebUI

> 依赖 Phase 10 (Task Manager)

### P12.1：HTTP 服务与 API 端点

| 属性 | 值 |
|------|-----|
| **目标** | 实现 WebUI 的 HTTP 服务和 3 个 API 端点 |
| **涉及文件** | `web/server.go`, `web/handler.go` |
| **实现要点** | 使用标准库 `net/http`；3 个端点：`POST /api/run`（创建任务 → 返回 run_id）、`GET /api/run/:id`（返回完整状态快照）、`POST /api/approval/:id`（提交审批决定）；`/` 路径通过 `go:embed` 返回静态文件；CORS 头；JSON 序列化 |
| **验证步骤** | 启动服务后 `curl -X POST http://localhost:8080/api/run -d '{"task":"test"}'` 返回 `{"run_id":"..."}` |
| **依赖** | P10.1, P6.5 |

### P12.2：WebUI 前端（HTML + CSS）

| 属性 | 值 |
|------|-----|
| **目标** | 实现 WebUI 前端页面 |
| **涉及文件** | `web/static/index.html`, `web/static/style.css` |
| **实现要点** | HTML：任务输入表单、Agent 状态展示区、Action Trace 列表、Governance 决策展示区、HITL 审批按钮、审计日志展示区；CSS：简洁的单页设计，无外部依赖（无 Bootstrap、Tailwind 等） |
| **验证步骤** | 打开 `http://localhost:8080` 页面正确渲染 |
| **依赖** | P12.1 |

### P12.3：WebUI 前端（JS 轮询 + 交互）

| 属性 | 值 |
|------|-----|
| **目标** | 实现前端轮询和 HITL 交互 |
| **涉及文件** | `web/static/app.js` |
| **实现要点** | 轮询：`setInterval` 每 1s 调用 `GET /api/run/:id` → 更新页面状态；HITL：点击 [Approve]/[Reject]/[MoreInfo] 按钮 → `POST /api/approval/:id`；状态更新：Agent 状态、迭代次数、Action 摘要、Observation 摘要、Governance 决策、审计日志 |
| **验证步骤** | 手动测试：启动 Agent 后页面每 1s 更新状态 |
| **依赖** | P12.1, P12.2 |

---

## Phase 13：集成测试与机制演示（5 个）

> 依赖所有前置 Phase

### P13.1：冷启动演示 — 系统从零生存性验证

| 属性 | 值 |
|------|-----|
| **目标** | 验证系统在完全空环境下（无 config、无 key、无 memory）从零启动后能完成一次完整且安全的 Agent 闭环 |
| **涉及文件** | `tests/demo/cold_start_test.go`（最终填充） |
| **实现要点** | 最终验证全部 8 个断言：Config 默认值、Credential degraded mode、Tool Registry 装载、Governance 评估、Agent Loop 完整闭环、Memory 空初始化、Runtime Task 生命周期、Audit 日志记录。使用 `Bootstrap()` + `NewTempEnvironment()` 辅助函数 |
| **验证步骤** | `go test -run TestColdStart ./tests/demo/... -v` 全部 8 个子测试通过 |
| **依赖** | P10.2 |

### P13.2：机制演示 ① — 治理护栏拦截危险动作

| 属性 | 值 |
|------|-----|
| **目标** | 实现可重复运行的机制演示：Governance 拦截危险 Action |
| **涉及文件** | `internal/governance/evaluator_test.go`（追加）或独立的 `demo/` 测试脚本 |
| **实现要点** | 构造 `execute_shell` Action 含 `git reset --hard HEAD~1`；调用 Governance.Evaluate()；断言 Decision = RequireApproval；断言 MatchedRules 包含 GIT-002；断言 Reasons 包含 "git reset --hard is irreversible" |
| **验证步骤** | `go test -run TestDemoGovernanceIntercept -v` 通过 |
| **依赖** | P6.7 |

### P13.3：机制演示 ② — 反馈闭环改变 Agent 行为

| 属性 | 值 |
|------|-----|
| **目标** | 实现可重复运行的机制演示：注入失败 → Agent 改变下一步 Action |
| **涉及文件** | `internal/agent/loop_test.go`（追加）或独立的 `demo/` 测试脚本 |
| **实现要点** | 控制变量法：same initial state + same Mock LLM decision script（函数模式）+ different Observation（Success:true vs Success:false）；断言：Observation:true 后 Agent 的下一步 Action 与 Observation:false 后不同；且第二轮 Context 中确实包含 Observation |
| **验证步骤** | `go test -run TestDemoFeedbackLoop -v` 通过 |
| **依赖** | P7.4, P8.3, P2.3 |

### P13.4：机制演示 ③ — 治理审计日志

| 属性 | 值 |
|------|-----|
| **目标** | 实现可重复运行的机制演示：审计日志包含完整决策链路 |
| **涉及文件** | `internal/governance/audit_test.go`（追加）或独立的 `demo/` 测试脚本 |
| **实现要点** | 完整流程：构造 Action → Governance.Evaluate → HITL.Approve → 断言 audit.jsonl 包含：Action 信息、Risk 等级、匹配规则、决策原因、审批结果、审批理由；脱敏检查：确保 params 中敏感字段被自动脱敏 |
| **验证步骤** | `go test -run TestDemoAuditLog -v` 通过 |
| **依赖** | P6.8, P6.5 |

### P13.5：端到端集成测试

| 属性 | 值 |
|------|-----|
| **目标** | 实现端到端的集成测试，验证完整 Harness 工作流 |
| **涉及文件** | 新增 `tests/e2e_test.go` |
| **实现要点** | 使用 MockProvider + 临时目录（模拟 workspace）：Agent 执行 3 轮迭代；每一轮有 Action → Governance → Tool → Observation；断言最终状态为 Done；断言总迭代次数正确；清理临时目录 |
| **验证步骤** | `go test ./tests/...` 或 `make test` 通过 |
| **依赖** | P13.1, P13.2, P13.3, P13.4 |

---

## 任务依赖图（完整）

```
P0.1 ─┬─ P0.2 ── P0.5 ── (后续填充)
      ├─ P0.3
      └─ P0.4

P1.1 ─┬─ P1.2 ─┬─ P5.1 ─┬─ P5.2 ──── P8.1 ─┐
      │        │        ├─ P5.3 ──── P8.2 ─┤│
      │        │        └─ P5.4 ──────┐    ││
      │        │                     │    ││
      │        │         P6.1 ─┬─ P6.2 ─┐│││
      │        │              │         ││││
      │        │         P6.3 ─┐        ││││
      │        │         P6.4 ─┤        ││││
      │        │         P6.5 ─┤        ││││
      │        │         P6.6 ─┤        ││││
      │        │              │         ││││
      │        │         P6.7 ◄─────────┘│││
      │        │         P6.8 ───────────┘││
      │        │                         ││
      │        │         P7.1 ◄── P9.2 ──┘│
      │        │         P7.2 ◄── P7.1    │
      │        │              │           │
      │        │         P7.3 ◄── P7.2    │
      │        │         P7.4 ◄── P7.3    │
      │        │              │           │
      │        │         P7.5 ◄── P7.4 + P8.3 + P9.1
      │        │              │
      │        │         P8.3 ◄── P7.2 + P8.1 + P8.2
      │        │
      │        └─ P1.4 ── P6.1~P6.8
      │
      ├─ P1.3 ── P8.1, P8.2
      ├─ P1.5 ── P9.1, P9.2, P9.3
      └─ P1.6 ── P3.1, P3.2

P2.1 ─┬─ P2.2 ── P2.3 ── P7.2
      └─ P2.4

P3.1 ── P3.2 ── P7.1

P4.1 ─┬─ P4.2
      ├─ P4.3 ── P6.8
      └─ P4.4 (Phase 2)

P9.1 ── P9.2 ── P7.1

P7.4 ── P10.1 ── P10.2 ── P12.1 ─┬─ P12.2
                                 └─ P12.3

P7.4 ── P11.1 ─┬─ P11.2 ── P11.3
               ├─ P11.4
               ├─ P11.5
               ├─ P11.6
               └─ P11.7

P10.2 ── P13.1 (Cold Start)
P13.1, P13.2, P13.3, P13.4 ── P13.5
```

---

## 可并行 worktree 建议

| Worktree | 包含 Phase | 预估 task 数 | 依赖 |
|----------|-----------|:-----------:|------|
| `wt-scaffold` | P0.1-P0.5 | 5 | 无 |
| `wt-core` | P1.1-P1.6 | 6 | 无 |
| `wt-llm` | P2.1-P2.4 | 4 | 无 |
| `wt-config` | P3.1-P3.2 | 2 | 无 |
| `wt-credential` | P4.1-P4.4 | 4 | 无 |
| `wt-tools` | P5.1-P5.4 | 4 | 无 |
| `wt-governance` | P6.1-P6.8 | 8 | P1.2, P1.4 |
| `wt-memory` | P9.1-P9.3 | 3 | 无 |
| `wt-loops` | P7.1-P7.5, P8.1-P8.3 | 8 | wt-llm, wt-config, wt-tools, wt-governance, wt-memory |
| `wt-runtime` | P10.1-P10.2 | 2 | wt-loops |
| `wt-cli` | P11.1-P11.7 | 7 | wt-loops, wt-credential, wt-memory |
| `wt-webui` | P12.1-P12.3 | 3 | wt-runtime |
| `wt-demo` | P13.1-P13.5 | 5 | 全部 |

**并行策略**：
- 第一波：`wt-scaffold` + `wt-core`（快，1-2 天）
- 第二波（并行）：`wt-llm`、`wt-config`、`wt-credential`、`wt-tools`、`wt-governance`、`wt-memory`（6 个 worktree 可同时进行）
- 第三波：`wt-loops`（需等待第二波完成，包含 P7.5 Cold Start 验证）
- 第四波：`wt-runtime` + `wt-cli`（可并行，均依赖 wt-loops）
- 第五波：`wt-webui` + `wt-demo`（依赖前面全部完成，包含 P13.1 Cold Start 最终验证）

---

## 常见问题

### Q：一个 task 约多长时间？

每个 task 设计为 2–5 分钟由 subagent 完成。复杂 task（如 P6.7 Evaluator 编排、P7.2 Agent Loop 状态机）可能需要 10–15 分钟，已经在实现要点中做了更细的子步骤拆分。

### Q：测试失败的情况怎么办？

TDD 流程：先写测试（红）→ 实现最少代码（绿）→ 重构。如果 subagent 提交的代码未通过测试，将失败信息作为 Feedback 回灌给 subagent 自行修正，修正后重新评审。

### Q：哪些 task 需要人工审批？

所有涉及 Governance 规则匹配的 task（P6.1-P6.8）在实现时可能需要人工判断规则设计是否合理。P7.2-P7.4 的 Agent Loop 需要人工验证循环逻辑是否正确。P13.1-P13.3 的机制演示需要人工确认演示结果是否符合预期。

### Q：PLAN 如何更新？

每完成一个 task，在 `PLAN.md` 中标记完成并附 commit hash。PLAN 持续更新，直到所有 task 完成。