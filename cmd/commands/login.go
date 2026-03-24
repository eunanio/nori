package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	nori "github.com/eunanio/nori/lib"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type loginOptions struct {
	username      string
	password      string
	passwordStdin bool
}

func newLoginCommand() *cobra.Command {
	opts := &loginOptions{}

	cmd := &cobra.Command{
		Use:   "login <registry>",
		Short: "Authenticate to a registry",
		Long: `Log in to an OCI registry.

Credentials can be provided via:
  - Interactive prompt (default)
  - Command line flags (--username, --password)
  - Standard input (--password-stdin)

Examples:
  # Interactive login
  nori login ghcr.io

  # Login with username and password
  nori login ghcr.io --username myuser --password mytoken

  # Login with password from stdin (for scripts)
  echo $TOKEN | nori login ghcr.io --username myuser --password-stdin

  # Login to Docker Hub
  nori login docker.io`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd, args, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.username, "username", "u", "", "Username for authentication")
	cmd.Flags().StringVarP(&opts.password, "password", "p", "", "Password or token for authentication")
	cmd.Flags().BoolVar(&opts.passwordStdin, "password-stdin", false, "Read password from stdin")

	return cmd
}

func runLogin(cmd *cobra.Command, args []string, opts *loginOptions) error {
	registry := args[0]

	// Resolve username (interactive prompt is CLI-only)
	username := opts.username
	if username == "" {
		fmt.Print("Username: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read username: %w", err)
		}
		username = strings.TrimSpace(input)
	}

	// Resolve password (interactive prompt is CLI-only)
	password := opts.password
	if password == "" {
		if opts.passwordStdin {
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read password from stdin: %w", err)
			}
			password = strings.TrimSpace(input)
		} else {
			fmt.Print("Password: ")
			passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
			fmt.Println()
			password = string(passwordBytes)
		}
	}

	if err := getLibClient().Login(cmd.Context(), registry, nori.Credentials{
		Username: username,
		Password: password,
	}); err != nil {
		return err
	}

	fmt.Printf("✓ Logged in to %s\n", registry)
	return nil
}
