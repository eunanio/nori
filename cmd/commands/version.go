package commands

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCommand(version, commit, date string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Display detailed version and build information for Nori.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Nori - OCI Module Management Tool\n")
			fmt.Printf("\n")
			fmt.Printf("Version:    %s\n", version)
			fmt.Printf("Commit:     %s\n", commit)
			fmt.Printf("Built:      %s\n", date)
			fmt.Printf("Go Version: %s\n", runtime.Version())
			fmt.Printf("OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}

	return cmd
}

