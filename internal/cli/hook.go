package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/myuon/track/internal/hooks"
	"github.com/myuon/track/internal/store/sqlite"
	"github.com/spf13/cobra"
)

func newHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage hooks",
	}
	cmd.AddCommand(newHookListCmd())
	cmd.AddCommand(newHookAddCmd())
	cmd.AddCommand(newHookRemoveCmd())
	cmd.AddCommand(newHookTestCmd())
	return cmd
}

func newHookListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			hooksList, err := store.ListHooks(ctx, "")
			if err != nil {
				return err
			}
			for _, h := range hooksList {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%s\n", h.ID, h.Event, h.RunCmd, h.CWD)
			}
			return nil
		},
	}
}

func newHookAddCmd() *cobra.Command {
	var runCmd string
	var cwd string

	cmd := &cobra.Command{
		Use:   "add <event>",
		Short: "Add hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runCmd == "" {
				return fmt.Errorf("--run is required")
			}
			if err := hooks.ValidateEvent(args[0]); err != nil {
				return err
			}

			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			if err := store.AddHook(ctx, args[0], runCmd, cwd); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&runCmd, "run", "", "Command to run")
	cmd.Flags().StringVar(&cwd, "cwd", "", "Working directory")
	return cmd
}

func newHookRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <hook_id>",
		Short: "Remove hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hookID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid hook_id: %s", args[0])
			}

			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			if err := store.RemoveHook(ctx, hookID); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
}

func newHookTestCmd() *cobra.Command {
	var issueID string
	cmd := &cobra.Command{
		Use:   "test <event>",
		Short: "Test hook event",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := hooks.ValidateEvent(args[0]); err != nil {
				return err
			}
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			if err := hooks.RunEvent(ctx, store, args[0], issueID); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&issueID, "issue", "", "Issue ID for context")
	return cmd
}
