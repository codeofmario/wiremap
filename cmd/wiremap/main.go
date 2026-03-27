package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	port       int
	devMode    bool
	configFile string
	hosts      []string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "wiremap",
		Short: "Visual Docker Network Explorer",
		Long:  "Wiremap is a self-hosted visual Docker network topology explorer with real-time log streaming, stats, and container inspection.",
		RunE:  run,
	}

	rootCmd.Flags().IntVarP(&port, "port", "p", 8080, "port to listen on")
	rootCmd.Flags().BoolVar(&devMode, "dev", false, "enable dev mode (proxy frontend to vite dev server)")
	rootCmd.Flags().StringVar(&configFile, "config", "", "path to wiremap.yml config file")
	rootCmd.Flags().StringSliceVar(&hosts, "host", nil, "Docker host URLs (can be repeated)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	app, err := InitializeApp(port, devMode, configFile, hosts)
	if err != nil {
		return fmt.Errorf("failed to initialize app: %w", err)
	}

	return app.Run()
}
