// Package downstream manages connections to downstream MCP servers,
// caching their tool definitions and proxying tool calls.
package downstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/TatoExp/multitool/internal/config"
)

// Manager holds active connections to downstream MCP servers and caches
// their tool metadata with prefixed names to avoid collisions.
type Manager struct {
	clients   map[string]*client.Client // server name -> MCP client
	tools     map[string]cachedTool     // prefixed name -> cached tool
	clientsMu sync.RWMutex
	toolsMu   sync.RWMutex
}

// cachedTool holds the original tool definition along with routing info.
type cachedTool struct {
	Server   string    // downstream server name
	Original string    // original (un-prefixed) tool name
	Tool     mcp.Tool  // full tool definition (schema, description...)
}

// NewManager reads the provided configuration, connects to every configured
// downstream server (stdio or HTTP/StreamableHTTP), and caches each one's
// tool list. Servers that fail to connect are logged and skipped; the shim
// continues to run with the servers that succeeded.
func NewManager(cfg *config.Config) (*Manager, error) {
	m := &Manager{
		clients: make(map[string]*client.Client),
		tools:   make(map[string]cachedTool),
	}

	if cfg == nil || len(cfg.MCPServers) == 0 {
		return m, nil
	}

	for name, srvCfg := range cfg.MCPServers {
		if err := m.connectServer(name, srvCfg); err != nil {
			log.Printf("[downstream] failed to connect %q: %v", name, err)
			continue
		}
		log.Printf("[downstream] connected %q (%d tools)", name, len(m.toolsForServer(name)))
	}

	return m, nil
}

// BuildInstructions returns a human-readable summary of all cached tools
// for use in the MCP server's Instructions field (added to the system prompt).
func (m *Manager) BuildInstructions() string {
	m.toolsMu.RLock()
	defer m.toolsMu.RUnlock()

	if len(m.tools) == 0 {
		return "The MCP Shim server has no downstream servers configured."
	}

	var b strings.Builder
	b.WriteString("You are connected to the MCP Shim server, which proxies tools from multiple downstream MCP servers.\n")
	b.WriteString("To use a tool, first call `get_schema` with the tool name to retrieve its JSON input schema.\n")
	b.WriteString("Then call `call_tool` with the tool name and the required arguments.\n\n")
	b.WriteString("Available tools:\n")

	for name, ct := range m.tools {
		desc := ct.Tool.Description
		if desc == "" {
			desc = "(no description)"
		}
		b.WriteString(fmt.Sprintf("- %s: %s\n", name, desc))
	}

	return b.String()
}

// GetTool looks up a tool by its prefixed name (e.g. "github__list_repos").
func (m *Manager) GetTool(prefixedName string) (cachedTool, bool) {
	m.toolsMu.RLock()
	defer m.toolsMu.RUnlock()
	t, ok := m.tools[prefixedName]
	return t, ok
}

// ListTools returns a sorted list of all available tools with their metadata.
func (m *Manager) ListTools() []struct {
	Name        string
	Description string
	Server      string
} {
	m.toolsMu.RLock()
	defer m.toolsMu.RUnlock()

	type ToolInfo struct {
		Name        string
		Description string
		Server      string
	}

	result := make([]ToolInfo, 0, len(m.tools))
	for name, ct := range m.tools {
		desc := ct.Tool.Description
		if desc == "" {
			desc = "(no description provided)"
		}
		result = append(result, ToolInfo{
			Name:        name,
			Description: desc,
			Server:      ct.Server,
		})
	}

	// Sort alphabetically by tool name for stable output.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	// Return using the anonymous struct type so the signature stays simple.
	out := make([]struct {
		Name        string
		Description string
		Server      string
	}, len(result))
	for i, r := range result {
		out[i] = struct {
			Name        string
			Description string
			Server      string
		}{
			Name:        r.Name,
			Description: r.Description,
			Server:      r.Server,
		}
	}
	return out
}

// CallTool proxies a tool invocation to the correct downstream server.
func (m *Manager) CallTool(ctx context.Context, prefixedName string, arguments any) (*mcp.CallToolResult, error) {
	ct, ok := m.GetTool(prefixedName)
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", prefixedName)
	}

	m.clientsMu.RLock()
	c, ok := m.clients[ct.Server]
	m.clientsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server %q not connected", ct.Server)
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      ct.Original,
			Arguments: arguments,
		},
	}

	return c.CallTool(ctx, req)
}

// GetSchemaJSON returns the input schema for a given prefixed tool as raw JSON.
func (m *Manager) GetSchemaJSON(prefixedName string) (json.RawMessage, error) {
	ct, ok := m.GetTool(prefixedName)
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", prefixedName)
	}

	// Return the raw input schema if available, otherwise marshal the
	// structured InputSchema field.
	if ct.Tool.RawInputSchema != nil {
		return ct.Tool.RawInputSchema, nil
	}

	schema, err := json.Marshal(ct.Tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("marshaling input schema: %w", err)
	}
	return schema, nil
}

// Close shuts down all downstream connections.
func (m *Manager) Close() error {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	for name, c := range m.clients {
		if err := c.Close(); err != nil {
			log.Printf("[downstream] error closing %q: %v", name, err)
		}
		delete(m.clients, name)
	}
	return nil
}

// ------------------------------------------------------------------
// Internal helpers
// ------------------------------------------------------------------

func (m *Manager) connectServer(name string, srvCfg *config.ServerConfig) error {
	var c *client.Client
	var err error

	switch srvCfg.Type {
	case "stdio":
		c, err = m.connectStdioServer(name, srvCfg)
	case "http", "streamable-http":
		c, err = m.connectHTTPServer(name, srvCfg)
	default:
		return fmt.Errorf("unsupported type %q", srvCfg.Type)
	}

	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "multitool",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	})
	if err != nil {
		c.Close()
		return fmt.Errorf("initializing: %w", err)
	}

	// Fetch and cache tools.
	listRes, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		c.Close()
		return fmt.Errorf("listing tools: %w", err)
	}

	m.clientsMu.Lock()
	m.clients[name] = c
	m.clientsMu.Unlock()

	m.toolsMu.Lock()
	for _, tool := range listRes.Tools {
		prefixed := fmt.Sprintf("%s__%s", name, tool.Name)
		m.tools[prefixed] = cachedTool{
			Server:   name,
			Original: tool.Name,
			Tool:     tool,
		}
	}
	m.toolsMu.Unlock()

	return nil
}

func (m *Manager) connectStdioServer(name string, srvCfg *config.ServerConfig) (*client.Client, error) {
	cmd := expandEnv(srvCfg.Command)
	args := make([]string, len(srvCfg.Args))
	for i, a := range srvCfg.Args {
		args[i] = expandEnv(a)
	}

	env := os.Environ()
	for k, v := range srvCfg.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, expandEnv(v)))
	}

	c, err := client.NewStdioMCPClient(cmd, env, args...)
	if err != nil {
		return nil, fmt.Errorf("starting stdio client: %w", err)
	}

	// Capture and log stderr from the downstream server.
	if r, ok := client.GetStderr(c); ok {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := r.Read(buf)
				if n > 0 {
					log.Printf("[downstream stderr %s] %s", name, strings.TrimSpace(string(buf[:n])))
				}
				if err != nil {
					if err != io.EOF {
						log.Printf("[downstream stderr %s] read error: %v", name, err)
					}
					return
				}
			}
		}()
	}

	return c, nil
}

func (m *Manager) connectHTTPServer(name string, srvCfg *config.ServerConfig) (*client.Client, error) {
	url := expandEnv(srvCfg.URL)

	// Prepare static headers.
	var opts []transport.StreamableHTTPCOption
	if len(srvCfg.Headers) > 0 {
		headers := make(map[string]string, len(srvCfg.Headers))
		for k, v := range srvCfg.Headers {
			headers[k] = expandEnv(v)
		}
		opts = append(opts, transport.WithHTTPHeaders(headers))
	}

	c, err := client.NewStreamableHttpClient(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP client: %w", err)
	}
	return c, nil
}

func (m *Manager) toolsForServer(serverName string) []string {
	m.toolsMu.RLock()
	defer m.toolsMu.RUnlock()
	var names []string
	for n, ct := range m.tools {
		if ct.Server == serverName {
			names = append(names, n)
		}
	}
	return names
}

// expandEnv replaces $VAR and ${VAR} with their environment values.
func expandEnv(s string) string {
	return os.ExpandEnv(s)
}
