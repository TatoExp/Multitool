package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// ------------------------------------------------------------------
// detectTransport
// ------------------------------------------------------------------

func TestDetectTransport(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{"http with URL", []string{"myserver", "https://example.com/mcp"}, "http", false},
		{"http with complex URL", []string{"s", "http://localhost:3000/api/v1/mcp"}, "http", false},
		{"stdio with command", []string{"myserver", "npx", "foo"}, "stdio", false},
		{"too few args", []string{"myserver"}, "", true},
		{"empty args", []string{}, "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectTransport(tc.args)
			if tc.wantErr {
				if got != "" {
					t.Fatalf("expected empty (uncertain), got %q", got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("detectTransport(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

// ------------------------------------------------------------------
// parseEnvFlags
// ------------------------------------------------------------------

func TestParseEnvFlags(t *testing.T) {
	tests := []struct {
		name    string
		flags   []string
		want    map[string]string
		wantErr bool
	}{
		{
			"simple key=value",
			[]string{"FOO=bar"},
			map[string]string{"FOO": "bar"},
			false,
		},
		{
			"multiple vars",
			[]string{"A=1", "B=two", "C=three four"},
			map[string]string{"A": "1", "B": "two", "C": "three four"},
			false,
		},
		{
			"value containing =",
			[]string{"KEY=val=ue"},
			map[string]string{"KEY": "val=ue"},
			false,
		},
		{
			"missing equals",
			[]string{"NOEQUALS"},
			nil,
			true,
		},
		{
			"empty list",
			[]string{},
			map[string]string{},
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEnvFlags(tc.flags)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tc.want))
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("for key %q, got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// ------------------------------------------------------------------
// parseHeaderFlags
// ------------------------------------------------------------------

func TestParseHeaderFlags(t *testing.T) {
	tests := []struct {
		name    string
		flags   []string
		want    map[string]string
		wantErr bool
	}{
		{
			"simple header",
			[]string{"Authorization: Bearer token"},
			map[string]string{"Authorization": "Bearer token"},
			false,
		},
		{
			"multiple headers",
			[]string{"A: 1", "B: two"},
			map[string]string{"A": "1", "B": "two"},
			false,
		},
		{
			"trims spaces",
			[]string{"  Name  :  Value  "},
			map[string]string{"Name": "Value"},
			false,
		},
		{
			"missing colon",
			[]string{"NOCOLON"},
			nil,
			true,
		},
		{
			"empty list",
			[]string{},
			map[string]string{},
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHeaderFlags(tc.flags)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tc.want))
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("for key %q, got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// ------------------------------------------------------------------
// runMCPAdd end-to-end tests (project scope, temp dir)
// ------------------------------------------------------------------

func TestRunMCPAdd_StdioNoDash(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cmd := &cobra.Command{}
	cmd.Flags().String("transport", "", "")
	cmd.Flags().StringArray("env", nil, "")
	cmd.Flags().StringArray("header", nil, "")
	cmd.Flags().String("scope", "project", "")

	os.Args = []string{"multitool", "mcp", "add", "playwright", "npx", "@playwright/mcp@latest"}
	err := runMCPAdd(cmd, []string{"playwright", "npx", "@playwright/mcp@latest"})
	if err != nil {
		t.Fatalf("runMCPAdd failed: %v", err)
	}

	// Verify the config file was created.
	cfgPath := filepath.Join(tmpDir, "multitool.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	content := string(data)
	if !contains(content, `"type": "stdio"`) {
		t.Fatalf("expected stdio type in config, got:\n%s", content)
	}
	if !contains(content, `"command": "npx"`) {
		t.Fatalf("expected npx command in config, got:\n%s", content)
	}
	if !contains(content, `"playwright"`) {
		t.Fatalf("expected playwright server name in config, got:\n%s", content)
	}
}

func TestRunMCPAdd_HttpAutoDetect(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cmd := &cobra.Command{}
	cmd.Flags().String("transport", "", "")
	cmd.Flags().StringArray("env", nil, "")
	cmd.Flags().StringArray("header", nil, "")
	cmd.Flags().String("scope", "project", "")

	err := runMCPAdd(cmd, []string{"github", "https://api.githubcopilot.com/mcp/"})
	if err != nil {
		t.Fatalf("runMCPAdd failed: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, "multitool.json")
	data, _ := os.ReadFile(cfgPath)
	content := string(data)
	if !contains(content, `"type": "http"`) {
		t.Fatalf("expected http type, got:\n%s", content)
	}
	if !contains(content, `"url": "https://api.githubcopilot.com/mcp/"`) {
		t.Fatalf("expected url, got:\n%s", content)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
