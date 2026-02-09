package cli

import (
	"fmt"
	"os"

	"github.com/myuon/track/internal/version"
	"github.com/spf13/cobra"
)

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "track",
		Short:         "CLI-first local issue tracker",
		Long:          "Track is a local-first task and issue tracker with optional automation and sync.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newConfigCmd())
	for _, c := range newIssueCommands() {
		cmd.AddCommand(c)
	}
	for _, c := range newIOCommands() {
		cmd.AddCommand(c)
	}
	cmd.AddCommand(newHookCmd())
	cmd.AddCommand(newGitHubCmd())
	cmd.AddCommand(newUICmd())

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.Version)
		},
	}
}
