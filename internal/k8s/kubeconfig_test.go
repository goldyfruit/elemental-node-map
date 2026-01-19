package k8s

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const sampleConfigTemplate = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.com
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: %s
current-context: %s
users:
- name: test
  user:
    token: dummy
`

func TestResolveKubeconfigFlagOverridesEnv(t *testing.T) {
	tempDir := t.TempDir()
	flagPath := writeConfig(t, tempDir, "flag.yaml", "flag-context")
	envPath := writeConfig(t, tempDir, "env.yaml", "env-context")

	t.Setenv("KUBECONFIG", envPath)

	_, info, err := ResolveKubeconfig(flagPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Source != "flag" {
		t.Fatalf("expected source flag, got %s", info.Source)
	}
	if info.Context != "flag-context" {
		t.Fatalf("expected flag context, got %s", info.Context)
	}
}

func TestResolveKubeconfigEnv(t *testing.T) {
	tempDir := t.TempDir()
	envPath := writeConfig(t, tempDir, "env.yaml", "env-context")
	t.Setenv("KUBECONFIG", envPath)

	_, info, err := ResolveKubeconfig("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Source != "env" {
		t.Fatalf("expected source env, got %s", info.Source)
	}
	if info.Context != "env-context" {
		t.Fatalf("expected env context, got %s", info.Context)
	}
}

func TestResolveKubeconfigContextOverride(t *testing.T) {
	tempDir := t.TempDir()
	path := writeConfig(t, tempDir, "config.yaml", "default-context")

	_, info, err := ResolveKubeconfig(path, "override-context")
	if err == nil {
		t.Fatalf("expected context error, got nil")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) || cfgErr.Kind != ErrContextNotFound {
		t.Fatalf("expected context not found error, got %v", err)
	}
	if info.Context != "override-context" {
		t.Fatalf("expected info context to be override-context, got %s", info.Context)
	}
}

func TestResolveKubeconfigNotFound(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	_, _, err := ResolveKubeconfig("", "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) || cfgErr.Kind != ErrKubeconfigNotFound {
		t.Fatalf("expected kubeconfig not found error, got %v", err)
	}
}

func TestResolveKubeconfigFromBytes(t *testing.T) {
	content := []byte(fmt.Sprintf(sampleConfigTemplate, "byte-context", "byte-context"))
	_, info, err := ResolveKubeconfigFromBytes(content, "rancher", []string{"cluster:c-123"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Source != "rancher" {
		t.Fatalf("expected source rancher, got %s", info.Source)
	}
	if info.Context != "byte-context" {
		t.Fatalf("expected context byte-context, got %s", info.Context)
	}
}

func TestResolveKubeconfigFromBytesMissingContext(t *testing.T) {
	content := []byte(fmt.Sprintf(sampleConfigTemplate, "byte-context", "byte-context"))
	_, _, err := ResolveKubeconfigFromBytes(content, "rancher", nil, "missing")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) || cfgErr.Kind != ErrContextNotFound {
		t.Fatalf("expected context not found error, got %v", err)
	}
}

func TestExtractServerAndToken(t *testing.T) {
	content := []byte(fmt.Sprintf(sampleConfigTemplate, "byte-context", "byte-context"))
	clientConfig, info, err := ResolveKubeconfigFromBytes(content, "inline", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	server, token, err := ExtractServerAndToken(clientConfig, info.Context)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if server != "https://example.com" {
		t.Fatalf("expected server https://example.com, got %s", server)
	}
	if token != "dummy" {
		t.Fatalf("expected token dummy, got %s", token)
	}
}

func TestExtractServerAndTokenMissingToken(t *testing.T) {
	content := []byte(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.com
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: ctx
current-context: ctx
users:
- name: test
  user: {}
`)
	clientConfig, info, err := ResolveKubeconfigFromBytes(content, "inline", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, _, err = ExtractServerAndToken(clientConfig, info.Context)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func writeConfig(t *testing.T, dir, name, context string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := []byte(fmt.Sprintf(sampleConfigTemplate, context, context))
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}
