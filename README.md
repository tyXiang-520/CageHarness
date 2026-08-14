# CageHarness — AI4SE Coding Agent Harness

> **Agent = LLM + Harness**
>
> LLM 相当于 CPU，只负责决定下一步做什么。Harness 是剩下的全部工程：把一个只会产生下一步设想的 LLM，封装成一台能稳定、可靠工作的系统。

CageHarness 是南京大学 AI4SE 课程期末项目。它从零实现了一个 Coding Agent 的 harness 内核——主循环、工具分发、治理护栏、反馈闭环、记忆系统——全部使用 Go 标准库，零外部依赖。

---

## 项目架构

```
cmd/harness          CLI 入口
  ↓
internal/cli/        CLI 薄封装（runtime client）
internal/web/        WebUI HTTP 服务（runtime client）
  ↓
internal/runtime/    AgentLoop + TaskManager（composition root）
  ↓
internal/governance/ 五层治理管线 ★（核心深度维度）
internal/llm/        LLM 抽象层 + MockProvider
internal/tools/      工具注册与执行（shell/file/test/git）
internal/feedback/   反馈解析器（shell/test → 结构化观察）
internal/memory/     文件存储 + 关键词检索
internal/agent/      状态机 + 观察类型
internal/protocol/   共享类型定义
internal/config/     配置加载
internal/credential/ 凭据安全存储
```

**架构原则**：所有 domain 包（agent/governance/tools/feedback/llm/memory）通过 `protocol` 共享类型，通过 `runtime` 在运行时组装。Domain 包之间互不依赖。CLI 和 WebUI 仅导入 `runtime` 包，不直接访问任何 domain 包。

---

## 快速开始

### 前置要求

- **Go 1.22+**（开发语言）
- **make**（可选，所有命令都有等价的 `go` 命令）

### 安装与运行

```bash
# 克隆仓库
git clone https://github.com/tyXiang-520/CageHarness.git
cd CageHarness

# 构建
go build -o build/harness ./cmd/harness/

# 运行测试
go test ./internal/... -count=1

# 运行 Demo 测试
go test ./tests/demo/ -v -count=1

# 运行 CLI
./build/harness run "say hello"
```

### 等价命令（无 make 时）

| make 命令 | 等价 go 命令 |
|-----------|-------------|
| `make build` | `go build -o build/harness ./cmd/harness/` |
| `make test` | `go test ./internal/... -count=1` |
| `make vet` | `go vet ./...` |
| `make tidy` | `go mod tidy` |

---

## 安全配置

### LLM API Key 管理

CageHarness 调用 LLM 需要 API Key。Key 绝不硬编码、不提交 Git、不写入日志。

**配置方式**：

1. 复制环境变量模板：
   ```bash
   cp .env.example .env
   ```

2. 编辑 `.env` 文件，填入你的 API Key：
   ```
   OPENAI_API_KEY=sk-your-key-here
   ```

3. **安全警告**：`.env` 文件为明文存储，已被 `.gitignore` 排除。请勿通过命令行 `export` 设置 Key（会进入 shell history）。生产环境建议使用操作系统钥匙串（Windows Credential Manager / macOS Keychain）。

**凭据威胁模型**：

| 威胁 | 对策 |
|------|------|
| Key 泄露到 Git | `.gitignore` 排除 `.env`、`.harness/`；pre-commit hook 检查 |
| Key 泄露到日志 | 审计日志自动脱敏敏感字段 |
| Key 泄露到 shell history | 引导用户使用 `.env` 文件而非 `export` |
| Key 泄露到进程列表 | 环境变量方案已知风险，生产环境建议 Keychain |

---

## 分发

### 二进制分发

```bash
# 构建当前平台二进制
go build -o build/harness ./cmd/harness/

# 跨平台构建
GOOS=linux GOARCH=amd64 go build -o build/harness-linux-amd64 ./cmd/harness/
GOOS=darwin GOARCH=arm64 go build -o build/harness-darwin-arm64 ./cmd/harness/
GOOS=windows GOARCH=amd64 go build -o build/harness-windows-amd64.exe ./cmd/harness/
```

### Docker 分发

```bash
docker build -t cageharness .
docker run -p 8080:8080 --env-file .env cageharness
```

### 已知限制

| 限制 | 说明 |
|------|------|
| 平台 | Windows/Linux/macOS（Go 交叉编译支持） |
| 架构 | amd64/arm64 |
| 依赖 | 无外部依赖（仅 Go 标准库） |
| LLM 供应商 | 当前使用 MockProvider；真实供应商接入需实现 `llm.Provider` 接口 |
| 凭据存储 | 当前支持 `.env` 文件；Windows Credential Manager 集成待实现 |

---

## 目录结构

```
CageHarness/
├── cmd/harness/main.go            CLI 入口
├── internal/
│   ├── agent/                     Agent 状态机 + 观察类型
│   ├── cli/                       CLI 薄封装
│   ├── config/                    配置加载
│   ├── credential/                凭据安全存储
│   ├── feedback/                  反馈解析器（shell/test）
│   ├── governance/                五层治理管线 ★
│   ├── llm/                       LLM 抽象层 + MockProvider
│   ├── memory/                    文件存储 + 关键词检索
│   ├── protocol/                  共享类型定义
│   ├── runtime/                   AgentLoop + TaskManager
│   ├── tools/                     工具注册与执行
│   └── web/                       WebUI HTTP 服务
├── tests/demo/                    集成 Demo 测试
├── web/static/                    WebUI 静态资源
├── build/                         构建产物
├── SPEC.md                        设计文档
├── PLAN.md                        实现计划
├── SPEC_PROCESS.md                过程文档
├── AGENT_LOG.md                   开发日志
├── REFLECTION.md                  反思报告
├── Dockerfile                     容器构建
├── Makefile                       构建脚本
├── .gitlab-ci.yml                 CI/CD 配置
├── config.example.yaml            配置模板
├── .env.example                   环境变量模板
└── .gitignore
```

---

## 核心机制

### 五层治理管线 ★（深度维度）

```
Action → SchemaValidator → RiskClassifier → PolicyEngine → Boundary → ExecutionController → Decision
```

| 层 | 功能 | 失败时 |
|----|------|--------|
| Schema | 验证 Action 结构合法性 | 直接 Deny |
| Risk | 判定操作风险等级（Low/Medium/High/Critical） | Critical → Deny，High → RequireApproval |
| Policy | 规则匹配（SHELL-001、FILE-001 等） | Deny |
| Boundary | 路径沙箱、资源限制 | Deny |
| Control | 执行超时、并发控制 | Escalate |

### 反馈闭环

```
Tool.Execute → ToolResult → FeedbackProcessor → FeedbackObservation → FormatForLLM → LLM 上下文
```

### 异步任务模型

```
POST /tasks → 202 Accepted + task_id → 后台 goroutine → AgentLoop
GET /tasks/{id} → 200 + status/result
```

---

## 运行 Demo

```bash
go test ./tests/demo/ -v -run "TestDemo" -count=1
```

| Demo | 测试 | 验证内容 |
|------|------|---------|
| Demo 1 | Cold Start | go build + go test + go vet 全通过 |
| Demo 2 | Governance Interception | ★ 危险命令被拦截，ExecutionCount==0 |
| Demo 3 | Feedback Loop | 完整消息链：system→user→assistant→tool |
| Demo 4 | Audit Trace | 治理审计 JSON 输出 |
| Demo 5 | End-to-End | 全链路集成验证 |

---

## 技术选型

| 维度 | 选择 | 理由 |
|------|------|------|
| 语言 | Go 1.22+ | 零依赖、静态编译、并发原生支持、单文件二进制分发 |
| LLM 供应商 | OpenAI / Anthropic（可替换） | 通过 `llm.Provider` 接口抽象 |
| 存储 | JSON 文件 | MVP 阶段无外部数据库依赖，可替换为 SQLite |
| 分发 | 二进制 + Docker | 二进制满足"单文件可执行"要求；Docker 满足容器化部署 |
| 测试 | Go 标准测试框架 | 无外部测试框架依赖，MockProvider 支持离线确定性测试 |

---

## 许可证

本项目为 AI4SE 课程期末项目，仅用于教育和学术目的。