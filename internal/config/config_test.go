package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigDir(t *testing.T) {
	dir := DefaultConfigDir()
	if dir == "" {
		t.Fatal("expected non-empty config dir")
	}
	// Should end with multitool
	if filepath.Base(dir) != "multitool" {
		t.Fatalf("expected path to end with multitool, got %s", dir)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if filepath.Base(path) != "config.json" {
		t.Fatalf("expected config.json, got %s", path)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	// Using a non-existent path should return an empty config.
	cfg, err := Load("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(cfg.MCPServers) != 0 {
		t.Fatalf("expected empty servers, got: %d", len(cfg.MCPServers))
	}
}

func TestLoadValidConfig(t *testing.T) {
	content := `{
		"mcpServers": {
			"github": {
				"type": "stdio",
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-github"],
				"env": {"GITHUB_TOKEN": "secret"}
			}
		}
	}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(cfg.MCPServers) != 1 {
		t.Fatalf("expected 1 server, got: %d", len(cfg.MCPServers))
	}
	github, ok := cfg.MCPServers["github"]
	if !ok {
		t.Fatal("expected github server in config")
	}
	if github.Type != "stdio" {
		t.Fatalf("expected type stdio, got %s", github.Type)
	}
	if github.Command != "npx" {
		t.Fatalf("expected command npx, got %s", github.Command)
	}
	if len(github.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(github.Args))
	}
	if github.Env["GITHUB_TOKEN"] != "secret" {
		t.Fatalf("expected GITHUB_TOKEN=secret, got %s", github.Env["GITHUB_TOKEN"])
	}
}

func TestLoadMissingType(t *testing.T) {
	content := `{"mcpServers": {"test": {"command": "foo"}}}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	_ = os.WriteFile(path, []byte(content), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing type")
	}
	if err.Error() != `server "test": missing type` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadMissingCommand(t *testing.T) {
	content := `{"mcpServers": {"test": {"type": "stdio"}}}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	_ = os.WriteFile(path, []byte(content), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if err.Error() != `server "test": missing command for stdio type` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadUnsupportedType(t *testing.T) {
	content := `{"mcpServers": {"test": {"type": "sse", "url": "http://example.com"}}}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	_ = os.WriteFile(path, []byte(content), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if err.Error() != `server "test": unsupported type "sse" (supported: stdio, http, streamable-http)` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadHttpConfig(t *testing.T) {
	content := `{
		"mcpServers": {
			"github": {
				"type": "streamable-http",
				"url": "https://api.githubcopilot.com/mcp/",
				"headers": {
					"Authorization": "Bearer ${GITHUB_TOKEN}"
				}
			}
		}
	}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(cfg.MCPServers) != 1 {
		t.Fatalf("expected 1 server, got: %d", len(cfg.MCPServers))
	}
	github, ok := cfg.MCPServers["github"]
	if !ok {
		t.Fatal("expected github server in config")
	}
	if github.Type != "streamable-http" {
		t.Fatalf("expected type streamable-http, got %s", github.Type)
	}
	if github.URL != "https://api.githubcopilot.com/mcp/" {
		t.Fatalf("expected url, got %s", github.URL)
	}
	if github.Headers["Authorization"] != "Bearer ${GITHUB_TOKEN}" {
		t.Fatalf("unexpected header: %s", github.Headers["Authorization"])
	}
}

func TestLoadHttpMissingURL(t *testing.T) {
	content := `{"mcpServers": {"test": {"type": "http"}}}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	_ = os.WriteFile(path, []byte(content), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if err.Error() != `server "test": missing url for http type` {
		t.Fatalf("unexpected error: %v", err)
	}
}
