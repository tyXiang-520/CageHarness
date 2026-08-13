// Package demo contains the cold start validation test suite.
//
// Cold Start 验证：一个全新的 Claude Code 会话仅凭 SPEC.md 和 PLAN.md
// 能否独立完成 Phase 0 的搭建。此测试文件包含真实断言（非 TODO 桩），
// 验证项目骨架的正确性。
package demo

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPhase0Scaffolding 验证 Phase 0 项目骨架完整性。
// 所有断言均为冷启动验证的一部分，确保新环境能正确搭建。
func TestPhase0Scaffolding(t *testing.T) {
	// 从测试文件位置推断项目根目录
	root := projectRoot(t)

	t.Run("go.mod exists", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err != nil {
			t.Fatalf("go.mod not found: %v", err)
		}
		content := string(data)
		if !contains(content, "module github.com/tyXiang-520/CageHarness") {
			t.Error("go.mod missing module declaration")
		}
		if !contains(content, "go 1.22") && !contains(content, "go 1.2") && !contains(content, "go 1.26") {
			t.Error("go.mod missing Go version directive")
		}
	})

	t.Run("cmd/harness/main.go exists and compiles", func(t *testing.T) {
		mainPath := filepath.Join(root, "cmd", "harness", "main.go")
		data, err := os.ReadFile(mainPath)
		if err != nil {
			t.Fatalf("cmd/harness/main.go not found: %v", err)
		}
		content := string(data)
		if !contains(content, "package main") {
			t.Error("main.go missing package main")
		}
		if !contains(content, "func main()") {
			t.Error("main.go missing func main()")
		}
	})

	t.Run("Makefile exists with build targets", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "Makefile"))
		if err != nil {
			t.Fatalf("Makefile not found: %v", err)
		}
		content := string(data)
		requiredTargets := []string{"build", "test", "clean", "vet", "tidy"}
		for _, target := range requiredTargets {
			if !contains(content, target+":") {
				t.Errorf("Makefile missing target: %s", target)
			}
		}
	})

	t.Run(".gitignore exists", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
		if err != nil {
			t.Fatalf(".gitignore not found: %v", err)
		}
		content := string(data)
		requiredEntries := []string{"harness", "build/", ".env", ".harness/"}
		for _, entry := range requiredEntries {
			if !contains(content, entry) {
				t.Errorf(".gitignore missing entry: %s", entry)
			}
		}
	})

	t.Run("config.example.yaml exists with governance section", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "config.example.yaml"))
		if err != nil {
			t.Fatalf("config.example.yaml not found: %v", err)
		}
		content := string(data)
		sections := []string{"llm:", "agent:", "governance:", "web:"}
		for _, section := range sections {
			if !contains(content, section) {
				t.Errorf("config.example.yaml missing section: %s", section)
			}
		}
	})

	t.Run(".env.example exists", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, ".env.example"))
		if err != nil {
			t.Fatalf(".env.example not found: %v", err)
		}
		if !contains(string(data), "OPENAI_API_KEY") {
			t.Error(".env.example missing OPENAI_API_KEY")
		}
	})

	t.Run("Dockerfile exists with multi-stage build", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
		if err != nil {
			t.Fatalf("Dockerfile not found: %v", err)
		}
		content := string(data)
		if !contains(content, "FROM golang") {
			t.Error("Dockerfile missing builder stage")
		}
		if !contains(content, "FROM alpine") {
			t.Error("Dockerfile missing runtime stage")
		}
	})

	t.Run(".gitlab-ci.yml exists", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, ".gitlab-ci.yml"))
		if err != nil {
			t.Fatalf(".gitlab-ci.yml not found: %v", err)
		}
		content := string(data)
		if !contains(content, "unit-test") {
			t.Error(".gitlab-ci.yml missing unit-test job")
		}
		if !contains(content, "docker-build") {
			t.Error(".gitlab-ci.yml missing docker-build job")
		}
	})

	t.Run("internal package directories exist", func(t *testing.T) {
		packages := []string{"agent", "llm", "tools", "governance", "feedback", "memory", "runtime", "config", "credential"}
		for _, pkg := range packages {
			docPath := filepath.Join(root, "internal", pkg, "doc.go")
			info, err := os.Stat(docPath)
			if err != nil {
				t.Errorf("internal/%s/doc.go not found: %v", pkg, err)
				continue
			}
			if info.Size() == 0 {
				t.Errorf("internal/%s/doc.go is empty", pkg)
			}
		}
	})

	t.Run("build directory exists", func(t *testing.T) {
		info, err := os.Stat(filepath.Join(root, "build"))
		if err != nil {
			t.Fatalf("build/ directory not found: %v", err)
		}
		if !info.IsDir() {
			t.Error("build/ is not a directory")
		}
	})

	t.Run("web/static directory exists", func(t *testing.T) {
		info, err := os.Stat(filepath.Join(root, "web", "static"))
		if err != nil {
			t.Fatalf("web/static/ directory not found: %v", err)
		}
		if !info.IsDir() {
			t.Error("web/static/ is not a directory")
		}
	})
}

// TestConfigExampleValidYAML 验证 config.example.yaml 是合法的 YAML 且可被解析。
func TestConfigExampleValidYAML(t *testing.T) {
	root := projectRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "config.example.yaml"))
	if err != nil {
		t.Fatalf("config.example.yaml not found: %v", err)
	}

	content := string(data)
	// 验证关键配置键出现在正确位置
	if !contains(content, "endpoint:") {
		t.Error("config.example.yaml missing endpoint under llm")
	}
	if !contains(content, "model:") {
		t.Error("config.example.yaml missing model under llm")
	}
	if !contains(content, "max_iterations:") {
		t.Error("config.example.yaml missing max_iterations under agent")
	}
	if !contains(content, "hitl_timeout:") {
		t.Error("config.example.yaml missing hitl_timeout under governance")
	}
	if !contains(content, "port:") {
		t.Error("config.example.yaml missing port under web")
	}
}

// TestSPECandPLANExist 验证核心设计文档存在。
func TestSPECandPLANExist(t *testing.T) {
	root := projectRoot(t)
	specPath := filepath.Join(root, "SPEC.md")
	planPath := filepath.Join(root, "PLAN.md")

	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		t.Error("SPEC.md not found — required design document")
	}
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		t.Error("PLAN.md not found — required implementation plan")
	}
}

// projectRoot 从测试文件位置向上查找 go.mod 来定位项目根目录。
func projectRoot(t *testing.T) string {
	t.Helper()
	// 测试文件在 tests/demo/ 下，向上两级即为项目根目录
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get working directory: %v", err)
	}
	// 如果在 tests/demo 目录，向上两级
	if filepath.Base(filepath.Dir(dir)) == "demo" {
		return filepath.Dir(filepath.Dir(dir))
	}
	// 否则尝试从当前目录向上找 go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found in any parent directory")
		}
		dir = parent
	}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && containsInner(s, substr)
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}