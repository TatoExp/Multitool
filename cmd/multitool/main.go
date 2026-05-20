package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/TatoExp/multitool/internal/config"
	"github.com/TatoExp/multitool/internal/downstream"
	"github.com/TatoExp/multitool/internal/proxy"
)

func main() {
	var stdioFlag bool
	flag.BoolVar(&stdioFlag, "stdio", false, "Run as a stdio MCP server")
	flag.Parse()

	if !stdioFlag {
		printUsage()
		os.Exit(0)
	}

	log.SetFlags(0) // Clean output for stdio MCP (no timestamps)

	// Load configuration.
	cfgPath := config.DefaultConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load configuration from %s: %v", cfgPath, err)
	}

	// Spin up downstream servers.
	dm, err := downstream.NewManager(cfg)
	if err != nil {
		log.Fatalf("Failed to initialise downstream manager: %v", err)
	}
	defer dm.Close()

	// Build and start the shim server.
	s := proxy.NewServer(dm)
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: multitool [flags]

  -stdio    Run as a stdio MCP server (required)

This is the MCP Shim server. It reads downstream MCP server definitions from:
  %s

Add servers to the config file in the same format as Claude Code's MCP configuration.
`, config.DefaultConfigPath())
}
