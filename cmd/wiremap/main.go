package main

import (
	"fmt"
	"os"

	"github.com/codeofmario/wiremap/internal/wiremap/config"
	"github.com/spf13/cobra"
)

var flags config.Flags

func main() {
	rootCmd := &cobra.Command{
		Use:   "wiremap",
		Short: "Visual Docker & Kubernetes Explorer",
		Long:  "Wiremap is a self-hosted visual Docker and Kubernetes topology explorer with real-time log streaming, stats, and container inspection.",
		RunE:  run,
	}

	rootCmd.Flags().IntVarP(&flags.Port, "port", "p", 7070, "port to listen on")
	rootCmd.Flags().BoolVar(&flags.DevMode, "dev", false, "enable dev mode (proxy frontend to vite dev server)")
	rootCmd.Flags().StringVar(&flags.ConfigFile, "config", "", "path to wiremap.yml config file")
	rootCmd.Flags().StringSliceVar(&flags.Hosts, "host", nil, "Docker host URLs (can be repeated)")
	rootCmd.Flags().StringVar(&flags.Kubeconfig, "kubeconfig", "", "path to a kubeconfig file (enables Kubernetes support)")
	rootCmd.Flags().StringSliceVar(&flags.KubeContexts, "kube-context", nil, "kubeconfig contexts to connect to (can be repeated)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	app, err := InitializeApp(flags)
	if err != nil {
		return fmt.Errorf("failed to initialize app: %w", err)
	}

	return app.Run()
}
