// Package config handles loading and parsing the MCP shim configuration
// from an OS-specific location. The configuration follows the same schema
// as Claude Code's MCP configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ServerConfig represents a single downstream MCP server configuration.
// This matches Claude Code's mcpServers schema.
type ServerConfig struct {
	// Type is the transport type. Supported: "stdio", "http", "streamable-http".
	Type string `json:"type"`
	// Command is the executable to run for stdio servers.
	Command string `json:"command,omitempty"`
	// Args are the arguments to pass to the command.
	Args []string `json:"args,omitempty"`
	// Env is a map of environment variables to set for the subprocess.
	Env map[string]string `json:"env,omitempty"`
	// URL is the endpoint URL for http/streamable-http servers.
	URL string `json:"url,omitempty"`
	// Headers is a map of HTTP headers for http/streamable-http servers.
	Headers map[string]string `json:"headers,omitempty"`
}

// Config represents the top-level configuration file structure.
type Config struct {
	// MCPServers is a map of server names to their configurations.
	MCPServers map[string]*ServerConfig `json:"mcpServers"`
}

// DefaultConfigDir returns the OS-specific directory for storing the
// MCP shim configuration.
func DefaultConfigDir() string {
	switch runtime.GOOS {
	case "darwin", "linux":
		if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
			return filepath.Join(xdgConfigHome, "multitool")
		}
		// On macOS and Linux, default to ~/.config
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(homeDir, ".config", "multitool")
	case "windows":
		// Use LOCALAPPDATA on Windows
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return ""
		}
		return filepath.Join(localAppData, "multitool")
	default:
		return ""
	}
}

// DefaultConfigPath returns the full path to the default configuration file.
func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.json")
}

// Load reads and parses the configuration from the given path.
// If the file does not exist, it returns an empty config (not an error).
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return an empty config if the file doesn't exist.
			return &Config{MCPServers: make(map[string]*ServerConfig)}, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]*ServerConfig)
	}

	// Validate configs.
	for name, server := range cfg.MCPServers {
		if server.Type == "" {
			return nil, fmt.Errorf("server %q: missing type", name)
		}
		switch server.Type {
		case "stdio":
			if server.Command == "" {
				return nil, fmt.Errorf("server %q: missing command for stdio type", name)
			}
		case "http", "streamable-http":
			if server.URL == "" {
				return nil, fmt.Errorf("server %q: missing url for %s type", name, server.Type)
			}
		default:
			return nil, fmt.Errorf("server %q: unsupported type %q (supported: stdio, http, streamable-http)", name, server.Type)
		}
	}

	return &cfg, nil
}

// EnsureDir creates the configuration directory if it doesn't exist.
func EnsureDir() error {
	dir := DefaultConfigDir()
	if dir == "" {
		return fmt.Errorf("could not determine config directory")
	}
	return os.MkdirAll(dir, 0o755)
}
