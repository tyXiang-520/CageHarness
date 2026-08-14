# CageHarness — AI4SE Coding Agent Harness

> **Agent = LLM + Harness**
>
> LLM 相当于 CPU，只负责决定下一步做什么。Harness 是剩下的全部工程：把一个只会产生下一步设想的 LLM，封装成一台能稳定、可靠工作的系统。

CageHarness 是南京大学 AI4SE 课程期末项目。它从零实现了一个 Coding Agent 的 harness 内核——主循环、工具分发、治理护栏、反馈闭环、记忆系统——全部使用 Go 标准库，零外部依赖。

---

## 📥 下载

| 平台 | 文件 | 下载 |
|------|------|------|
| Windows (amd64) | `harness-windows-amd64.exe` | [下载](https://github.com/tyXiang-520/CageHarness/releases/latest/download/harness-windows-amd64.exe) |
| macOS (Intel) | `harness-darwin-amd64` | [下载](https://github.com/tyXiang-520/CageHarness/releases/latest/download/harness-darwin-amd64) |
| macOS (Apple Silicon) | `harness-darwin-arm64` | [下载](https://github.com/tyXiang-520/CageHarness/releases/latest/download/harness-darwin-arm64) |
| Linux (amd64) | `harness-linux-amd64` | [下载](https://github.com/tyXiang-520/CageHarness/releases/latest/download/harness-linux-amd64) |
| Linux (arm64) | `harness-linux-arm64` | [下载](https://github.com/tyXiang-520/CageHarness/releases/latest/download/harness-linux-arm64) |
| Docker | `ghcr.io/tyxiang-520/cageharness` | `docker pull ghcr.io/tyxiang-520/cageharness:latest` |

🌐 **在线 Demo**：[SCF 香港节点](https://1468764621-lg1ve1o8np.ap-hongkong.tencentscf.com)

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
internal/credential/ 凭据安全存储（OS Keychain + 交互式引导）
```

**架构原则**：所有 domain 包（agent/governance/tools/feedback/llm/memory）通过 `protocol` 共享类型，通过 `runtime` 在运行时组装。Domain 包之间互不依赖。CLI 和 WebUI 仅导入 `runtime` 包，不直接访问任何 domain 包。

---

## 快速开始

### 前置要求

- **Go 1.23+**（开发语言）
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
| `make build-all` | 跨平台构建 5 个目标 |
| `make build-scf` | SCF 静态二进制构建 |
| `make test` | `go test ./internal/... -count=1` |
| `make vet` | `go vet ./...` |
| `make tidy` | `go mod tidy` |

---

## 凭据管理

### 交互式 API Key 配置

CageHarness 提供交互式引导录入，绝不硬编码、不提交 Git、不写入日志、不显示明文。

```bash
# 交互式引导录入（输入隐藏，Ctrl+C 取消）
./harness key setup

# 查看已配置的 Key（显示脱敏信息）
./harness key status

# 清除已存储的 Key
./harness key clear OPENAI_API_KEY
```

### 存储层级

| 优先级 | 存储方式 | 说明 |
|--------|---------|------|
| 1 | OS 原生密钥链 | Windows Credential Manager / macOS Keychain / Linux Secret Service |
| 2 | 环境变量 / `.env` | 回退方案，`.env` 已被 `.gitignore` 排除 |

### 凭据威胁模型

| 威胁 | 对策 | 状态 |
|------|------|------|
| Key 泄露到 Git | `.gitignore` 排除 `.env`、`.harness/` | ✅ |
| Key 泄露到日志 | 审计日志自动脱敏敏感字段 | ✅ |
| Key 泄露到 shell history | 交互式引导录入，无需 `export` | ✅ |
| Key 泄露到进程列表 | OS 密钥链存储，环境变量仅作回退 | ✅ |
| Key 泄露到终端 | 输入时隐藏回显（Windows/macOS/Linux） | ✅ |
| Key 明文存储 | Windows DPAPI / macOS Keychain 加密 | ✅ |
| 凭据残留 | `harness key clear` 清理 | ✅ |

---

## CLI 命令参考

```
harness run     <task>       同步运行任务
harness submit  <task>       异步提交任务
harness status  <task-id>    查看任务状态
harness list                 列出所有任务
harness cancel  <task-id>    取消任务
harness serve   [port]       启动 WebUI 服务器（默认 :8080）
harness key     <command>    凭据管理（setup / status / clear）
```

---

## 分发

### 二进制分发

从 [GitHub Releases](https://github.com/tyXiang-520/CageHarness/releases) 下载预编译二进制。

```bash
# 本地跨平台构建
make build-all      # 5 平台
make build-scf      # SCF 静态二进制
```

### Docker 分发

```bash
docker pull ghcr.io/tyxiang-520/cageharness:latest
docker run -p 8080:8080 ghcr.io/tyxiang-520/cageharness:latest
```

### 部署到腾讯云 SCF

1. 下载 `harness-scf` 二进制（GitHub Releases）
2. 在腾讯云控制台创建 SCF Web 函数（香港节点）
3. 上传二进制，设置环境变量 `PORT=9000`
4. 获取公网 URL 即可访问

---

## 目录结构

```
CageHarness/
├── cmd/harness/main.go            CLI 入口
├── internal/
│   ├── agent/                     Agent 状态机 + 观察类型
│   ├── cli/                       CLI 薄封装
│   ├── config/                    配置加载
│   ├── credential/                凭据安全存储（Keychain + Env）
│   ├── feedback/                  反馈解析器（shell/test）
│   ├── governance/                五层治理管线 ★
│   ├── llm/                       LLM 抽象层 + MockProvider
│   ├── memory/                    文件存储 + 关键词检索
│   ├── protocol/                  共享类型定义
│   ├── runtime/                   AgentLoop + TaskManager
│   ├── tools/                     工具注册与执行
│   └── web/                       WebUI HTTP 服务 + 内嵌静态资源
├── tests/demo/                    集成 Demo 测试
├── docs/                          文档（COS 部署页面）
├── .github/workflows/             CI/CD（Release 自动发布）
├── build/                         构建产物
├── SPEC.md                        设计文档
├── Dockerfile                     容器构建
├── Makefile                       构建脚本
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

### WebUI 治理可视化

WebUI 提供完整的治理管道可视化：
- **Action Trace**：状态转换时间线（彩色标记）
- **Audit Trail**：每层管道的 pass/fail 状态
- **Risk Level**：低/中/高/严重 风险徽章
- **Decision**：Allow/Deny/RequireApproval/Escalate 决策徽章

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
| 语言 | Go 1.23+ | 零依赖、静态编译、并发原生支持、单文件二进制分发 |
| LLM 供应商 | OpenAI / Anthropic（可替换） | 通过 `llm.Provider` 接口抽象 |
| 存储 | JSON 文件 | MVP 阶段无外部数据库依赖，可替换为 SQLite |
| 分发 | 二进制 + Docker + GitHub Releases | 二进制满足"单文件可执行"要求；Docker 满足容器化部署 |
| 部署 | 腾讯云 COS + SCF（香港） | 无需备案、国内访问快、免费额度充足 |
| 测试 | Go 标准测试框架 | 无外部测试框架依赖，MockProvider 支持离线确定性测试 |

---

## 许可证

本项目为 AI4SE 课程期末项目，仅用于教育和学术目的。