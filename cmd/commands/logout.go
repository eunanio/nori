package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLogoutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout <registry>",
		Short: "Remove registry credentials",
		Long: `Log out from an OCI registry by removing stored credentials.

Examples:
  # Logout from a registry
  nori logout ghcr.io

  # Logout from Docker Hub
  nori logout docker.io`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogout(cmd, args)
		},
	}

	return cmd
}

func runLogout(cmd *cobra.Command, args []string) error {
	registry := args[0]

	if err := getLibClient().Logout(cmd.Context(), registry); err != nil {
		return err
	}

	fmt.Printf("✓ Logged out from %s\n", registry)
	return nil
}
