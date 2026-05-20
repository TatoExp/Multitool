package cmd

import (
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	"github.com/TatoExp/multitool/internal/config"
	"github.com/TatoExp/multitool/internal/downstream"
	"github.com/TatoExp/multitool/internal/proxy"
)

var stdioFlag bool

// rootCmd is the entry point for the CLI.
var rootCmd = &cobra.Command{
	Use:   "multitool",
	Short: "MCP server shim — proxies tools from many servers through a tiny surface",
	RunE: func(c *cobra.Command, args []string) error {
		if stdioFlag {
			return runStdioServer()
		}
		return c.Usage()
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&stdioFlag, "stdio", false, "Run as a stdio MCP server")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func runStdioServer() error {
	log.SetFlags(0) // Clean output for stdio MCP (no timestamps)

	cfgPath := config.DefaultConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration from %s: %w", cfgPath, err)
	}

	dm, err := downstream.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialise downstream manager: %w", err)
	}
	defer dm.Close()

	s := proxy.NewServer(dm)
	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}
