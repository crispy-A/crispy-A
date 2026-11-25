package config

import (
	"os"
	"path/filepath"
	"testing"
)

// 回归：Windows 编辑器写入的 UTF-8 BOM 曾让 godotenv 静默失败，导致 .env 全部未生效。
func TestIsTemplateValue(t *testing.T) {
	templateValues := []string{
		"replace-with-a-random-secret-at-least-32-chars",
		"change-me-please",
		"changeme",
		"your-secret",
	}
	for _, value := range templateValues {
		if !isTemplateValue(value) {
			t.Fatalf("expected %q to be treated as a template value", value)
		}
	}

	safeValues := []string{
		"0Yb9k2mQ8sV1xR4tL6nP3cF7hJ5dG0aZ",
		"CrispyAdmin2026!",
	}
	for _, value := range safeValues {
		if isTemplateValue(value) {
			t.Fatalf("expected %q to be accepted", value)
		}
	}
}

func TestLoadDotEnvAcceptsUTF8BOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "\uFEFFCRISPY_BOM_TEST=ok\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("load .env: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("CRISPY_BOM_TEST") })

	if got := os.Getenv("CRISPY_BOM_TEST"); got != "ok" {
		t.Fatalf("expected CRISPY_BOM_TEST=ok, got %q", got)
	}
}

func TestLoadDotEnvKeepsExistingEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("CRISPY_PRECEDENCE_TEST=from-file\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	t.Setenv("CRISPY_PRECEDENCE_TEST", "from-process")

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("load .env: %v", err)
	}
	if got := os.Getenv("CRISPY_PRECEDENCE_TEST"); got != "from-process" {
		t.Fatalf("process environment must win, got %q", got)
	}
}
