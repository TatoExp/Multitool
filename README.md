# MCP Shim

An MCP server shim that pre-loads downstream MCP servers and proxies their tools, exposing only two fixed tools (`get_schema`, `call_tool`) to save context space.

## Install

### With `go install`

```bash
go install github.com/TatoExp/multitool@latest
```

### With Claude Code

```bash
claude mcp add --transport stdio multitool -- go run github.com/TatoExp/multitool@latest --stdio
```

Then create your downstream server config at `~/.config/multitool/config.json` and Claude Code will load it automatically.

## How It Works

Instead of loading all tool schemas into the AI context (which can consume thousands of tokens), the shim exposes only two fixed tools:

- `get_schema` — retrieve the JSON input schema for any downstream tool.
- `call_tool` — invoke a downstream tool by name with arguments.

The **descriptions of these two tools embed the full list of available downstream tools**, so the AI sees them immediately in the `tools/list` response without needing an extra discovery round-trip.

## Configuration

Create `~/.config/multitool/config.json` (macOS/Linux) or the equivalent OS-specific path:

```json
{
  "mcpServers": {
    "github": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"]
    },
    "filesystem": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    }
  }
}
```

The format is identical to Claude Code's MCP configuration (`~/.claude.json`). The shim supports `stdio`, `http`, and `streamable-http` transports.

### Stdio Example

```json
{
  "mcpServers": {
    "filesystem": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    }
  }
}
```

### HTTP Example

```json
{
  "mcpServers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      }
    }
  }
}
```

For `http` transports:
- `url` — the endpoint URL (required).
- `headers` — static HTTP headers sent with every request. Environment variables are expanded (`${VAR}`).

Environment variable expansion works for all transports.

When Claude Code initializes, the shim exposes only `get_schema` and `call_tool`. The AI sees the available downstream tools directly in their descriptions:

1. Claude calls `get_schema` with `tool_name: "github__list_repos"`.
2. The shim returns the JSON schema.
3. Claude then calls `call_tool` with `tool_name: "github__list_repos"` and the arguments.
4. The shim proxies the call to the downstream GitHub server and returns the result.

## Available Tools from the Shim

| Tool | Description |
|------|-------------|
| `get_schema` | Returns the JSON input schema for a specific tool. Its description contains the full list of available tools so the AI knows what to ask for. |
| `call_tool` | Invokes the actual downstream tool with arguments. Its description also contains the full list of available tools. |

## Tool Name Prefixing

All downstream tools are prefixed with their server name and a double underscore to avoid collisions:

- `github__list_repos`
- `github__create_issue`
- `filesystem__list_directory`

## CLI Commands

### Run the MCP Server

```bash
# Required for Claude Code / OpenCode
./multitool --stdio
```

### Manage Downstream Servers

The `mcp` subcommand lets you add, list, inspect, and remove downstream servers using the same syntax as `claude mcp add`.

```bash
# Add a stdio server (auto-detected)
multitool mcp add playwright npx @playwright/mcp@latest

# Add a stdio server with env vars and command flags (use -- before the command if flags conflict)
multitool mcp add -e KEY=val myserver -- npx -y @modelcontextprotocol/server-github

# Add an HTTP server (auto-detected)
multitool mcp add myserver https://api.githubcopilot.com/mcp/

# Add an HTTP server with custom headers
multitool mcp add -H "Authorization: Bearer token" myserver https://api.example.com/mcp
```

**Scopes**
- `--scope local` (default): stores in `~/.config/multitool/config.json`
- `--scope project`: stores in `./multitool.json` in the current directory

You can also hand-edit the JSON config directly if you prefer.

## Project Structure

```
.
├── cmd/
├── main.go                  # Entry point
├── cmd/
│   ├── root.go              # Root command + --stdio flag
│   └── mcp.go               # mcp add/list/get/remove
├── internal/
│   ├── config/
│   │   ├── config.go        # Config loading (Claude Code schema)
│   │   └── config_test.go   # Unit tests
│   ├── downstream/
│   │   └── manager.go       # Spawn servers, cache tools, proxy calls
│   └── proxy/
│       └── proxy.go         # MCP shim server (get_schema, call_tool)
├── go.mod
└── README.md
```

## Future Enhancements

- env var expansion with `${VAR:-default}` fallback syntax.
- Graceful reconnection when downstream servers crash.

## License

MIT
