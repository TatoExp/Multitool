package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TatoExp/multitool/internal/config"
)

func init() {
	rootCmd.AddCommand(mcpCmd)
}

// ------------------------------------------------------------------
// mcp parent command
// ------------------------------------------------------------------

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage downstream MCP servers",
}

// ------------------------------------------------------------------
// mcp add
// ------------------------------------------------------------------

var mcpAddCmd = &cobra.Command{
	Use:   "add [flags] <name> [-- <command> [args...]] | add [flags] <name> <url>",
	Short: "Add a downstream MCP server",
	Example: `  # Stdio server
  multitool mcp add --transport stdio --env KEY=val myserver -- npx -y @modelcontextprotocol/server-github

  # HTTP server
  multitool mcp add --transport http --header "Authorization: Bearer token" myserver https://api.example.com/mcp`,
	RunE: runMCPAdd,
}

func init() {
	mcpCmd.AddCommand(mcpAddCmd, mcpAddJSONCmd, mcpListCmd, mcpGetCmd, mcpRemoveCmd)

	mcpAddCmd.Flags().StringP("transport", "t", "", "Transport type: stdio, http, or streamable-http")
	_ = mcpAddCmd.MarkFlagRequired("transport")
	mcpAddCmd.Flags().StringArrayP("env", "e", nil, "Environment variable in KEY=value form (stdio only, repeatable)")
	mcpAddCmd.Flags().StringArrayP("header", "H", nil, "HTTP header in 'Name: Value' form (http only, repeatable)")
	addScopeFlag(mcpAddCmd)
}

func runMCPAdd(c *cobra.Command, args []string) error {
	transportType, _ := c.Flags().GetString("transport")
	envFlags, _ := c.Flags().GetStringArray("env")
	headerFlags, _ := c.Flags().GetStringArray("header")
	scope, _ := c.Flags().GetString("scope")

	cfgPath, err := scopeConfigPath(scope)
	if err != nil {
		return err
	}

	// Find the raw '--' separator in os.Args so we can distinguish server
	// name from stdio command / args.
	dashIdx := -1
	for i := range os.Args {
		if os.Args[i] == "--" {
			dashIdx = i
			break
		}
	}

	var name string
	var serverCfg *config.ServerConfig

	switch transportType {
	case "stdio":
		if dashIdx < 0 {
			return errors.New("stdio servers require '--' followed by <command> [args...]")
		}
		if len(args) == 0 {
			return errors.New("server name is required")
		}
		name = args[0]

		cmdStart := dashIdx + 1
		if cmdStart >= len(os.Args) {
			return errors.New("command is required after '--'")
		}
		command := os.Args[cmdStart]
		commandArgs := os.Args[cmdStart+1:]

		envMap, err := parseEnvFlags(envFlags)
		if err != nil {
			return err
		}

		serverCfg = &config.ServerConfig{
			Type:    "stdio",
			Command: command,
			Args:    commandArgs,
			Env:     envMap,
		}

	case "http", "streamable-http":
		if len(args) < 2 {
			return errors.New("<name> and <url> are required for http servers")
		}
		name = args[0]
		url := args[len(args)-1]

		headerMap, err := parseHeaderFlags(headerFlags)
		if err != nil {
			return err
		}

		serverCfg = &config.ServerConfig{
			Type:    "http",
			URL:     url,
			Headers: headerMap,
		}

	default:
		return fmt.Errorf("unsupported transport %q", transportType)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	cfg.MCPServers[name] = serverCfg
	if err := cfg.Save(cfgPath); err != nil {
		return err
	}

	fmt.Printf("Added server %q (%s)\n", name, transportType)
	return nil
}

// ------------------------------------------------------------------
// mcp add-json
// ------------------------------------------------------------------

var mcpAddJSONCmd = &cobra.Command{
	Use:   "add-json [flags] <name> <json>",
	Short: "Add a downstream MCP server from an inline JSON configuration",
	Example: `  # HTTP server
  multitool mcp add-json myserver '{"type":"http","url":"https://api.example.com/mcp","headers":{"Authorization":"Bearer token"}}'`,
	Args: cobra.ExactArgs(2),
	RunE: runMCPAddJSON,
}

func init() {
	addScopeFlag(mcpAddJSONCmd)
}

func runMCPAddJSON(c *cobra.Command, args []string) error {
	name := args[0]
	jsonStr := args[1]
	scope, _ := c.Flags().GetString("scope")

	cfgPath, err := scopeConfigPath(scope)
	if err != nil {
		return err
	}

	var serverCfg config.ServerConfig
	if err := json.Unmarshal([]byte(jsonStr), &serverCfg); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	// Normalise aliases
	if serverCfg.Type == "streamable-http" {
		serverCfg.Type = "http"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	cfg.MCPServers[name] = &serverCfg
	if err := cfg.Save(cfgPath); err != nil {
		return err
	}

	fmt.Printf("Added server %q (%s)\n", name, serverCfg.Type)
	return nil
}

// ------------------------------------------------------------------
// mcp list
// ------------------------------------------------------------------

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured downstream MCP servers",
	RunE:  runMCPList,
}

func init() {
	addScopeFlag(mcpListCmd)
}

func runMCPList(c *cobra.Command, args []string) error {
	scope, _ := c.Flags().GetString("scope")
	cfgPath, err := scopeConfigPath(scope)
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	if len(cfg.MCPServers) == 0 {
		fmt.Println("No MCP servers configured.")
		return nil
	}

	for name, srv := range cfg.MCPServers {
		fmt.Printf("%s\t%s", name, srv.Type)
		if srv.Type == "stdio" {
			fmt.Printf("\t%s %s", srv.Command, strings.Join(srv.Args, " "))
		} else {
			fmt.Printf("\t%s", srv.URL)
		}
		fmt.Println()
	}
	return nil
}

// ------------------------------------------------------------------
// mcp get
// ------------------------------------------------------------------

var mcpGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Show details for a configured MCP server",
	Args:  cobra.ExactArgs(1),
	RunE:  runMCPGet,
}

func init() {
	addScopeFlag(mcpGetCmd)
}

func runMCPGet(c *cobra.Command, args []string) error {
	name := args[0]
	scope, _ := c.Flags().GetString("scope")
	cfgPath, err := scopeConfigPath(scope)
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	srv, ok := cfg.MCPServers[name]
	if !ok {
		return fmt.Errorf("server %q not found", name)
	}

	data, err := json.MarshalIndent(srv, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// ------------------------------------------------------------------
// mcp remove
// ------------------------------------------------------------------

var mcpRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a configured MCP server",
	Args:  cobra.ExactArgs(1),
	RunE:  runMCPRemove,
}

func init() {
	addScopeFlag(mcpRemoveCmd)
}

func runMCPRemove(c *cobra.Command, args []string) error {
	name := args[0]
	scope, _ := c.Flags().GetString("scope")
	cfgPath, err := scopeConfigPath(scope)
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	if _, ok := cfg.MCPServers[name]; !ok {
		return fmt.Errorf("server %q not found", name)
	}

	delete(cfg.MCPServers, name)
	if err := cfg.Save(cfgPath); err != nil {
		return err
	}

	fmt.Printf("Removed server %q\n", name)
	return nil
}

// ------------------------------------------------------------------
// Helpers
// ------------------------------------------------------------------

func addScopeFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("scope", "s", "local", "Config scope: local (default) or project")
}

func scopeConfigPath(scope string) (string, error) {
	switch scope {
	case "local", "user":
		return config.DefaultConfigPath(), nil
	case "project":
		return "multitool.json", nil
	default:
		return "", fmt.Errorf("unsupported scope %q (supported: local, project)", scope)
	}
}

func parseEnvFlags(flags []string) (map[string]string, error) {
	m := make(map[string]string, len(flags))
	for _, f := range flags {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid env format %q (expected KEY=value)", f)
		}
		m[parts[0]] = parts[1]
	}
	return m, nil
}

func parseHeaderFlags(flags []string) (map[string]string, error) {
	m := make(map[string]string, len(flags))
	for _, f := range flags {
		parts := strings.SplitN(f, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header format %q (expected 'Name: Value')", f)
		}
		m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return m, nil
}
