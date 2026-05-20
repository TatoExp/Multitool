// Package proxy implements the MCP shim server. It exposes three fixed
// tools (list_available_tools, get_schema, call_tool) and routes
// downstream tool invocations through the pre-connected downstream manager.
package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/TatoExp/multitool/internal/downstream"
)

// NewServer creates the MCP shim server with instructions and proxy tools.
// The list of available downstream tools is embedded directly into the
// descriptions so the AI sees them immediately without a discovery round-trip.
func NewServer(dm *downstream.Manager) *server.MCPServer {
	instructions := dm.BuildInstructions()

	tools := dm.ListTools()
	toolCatalog := buildToolCatalog(tools)

	s := server.NewMCPServer(
		"multitool",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithInstructions(instructions),
	)

	// Register get_schema tool — description includes the catalog so the AI
	// knows which tools it can ask schemas for.
	getSchemaDesc := "Retrieve the JSON input schema for a downstream tool.\n\n" + toolCatalog
	getSchemaTool := mcp.NewTool("get_schema",
		mcp.WithDescription(getSchemaDesc),
		mcp.WithString("tool_name",
			mcp.Required(),
			mcp.Description("The prefixed name of the tool (e.g. github__list_repos)"),
		),
	)
	s.AddTool(getSchemaTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toolName, err := req.RequireString("tool_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		schemaJSON, err := dm.GetSchemaJSON(toolName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var schemaObj map[string]any
		if err := json.Unmarshal(schemaJSON, &schemaObj); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("parsing schema: %v", err)), nil
		}

		response := map[string]any{
			"tool_name":   toolName,
			"inputSchema": schemaObj,
		}
		respJSON, _ := json.MarshalIndent(response, "", "  ")
		return mcp.NewToolResultText(string(respJSON)), nil
	})

	// Register call_tool tool — description includes the catalog so the AI
	// knows which tools it can invoke.
	callToolDesc := "Invoke a downstream tool by name with arguments.\n\n" + toolCatalog
	callToolTool := mcp.NewTool("call_tool",
		mcp.WithDescription(callToolDesc),
		mcp.WithString("tool_name",
			mcp.Required(),
			mcp.Description("The prefixed name of the tool to call (e.g. github__list_repos)"),
		),
		mcp.WithAny("arguments",
			mcp.Required(),
			mcp.Description("A JSON object containing the arguments to pass to the tool"),
		),
	)
	s.AddTool(callToolTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		toolName, err := req.RequireString("tool_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		rawArgs := req.GetArguments()["arguments"]
		if rawArgs == nil {
			return mcp.NewToolResultError("arguments are required"), nil
		}

		var args any = rawArgs
		if strArgs, ok := rawArgs.(string); ok {
			if err := json.Unmarshal([]byte(strArgs), &args); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid arguments JSON: %v", err)), nil
			}
		}

		result, err := dm.CallTool(ctx, toolName, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return result, nil
	})

	return s
}

// buildToolCatalog turns the cached tool list into a compact string block
// suitable for embedding in a tool description.
func buildToolCatalog(tools []struct {
	Name        string
	Description string
	Server      string
}) string {
	if len(tools) == 0 {
		return "No downstream tools are currently available."
	}
	var b strings.Builder
	b.WriteString("Available tools:\n")
	for _, t := range tools {
		desc := t.Description
		if desc == "" {
			desc = "(no description)"
		}
		b.WriteString(fmt.Sprintf("- %s (%s): %s\n", t.Name, t.Server, desc))
	}
	return b.String()
}
