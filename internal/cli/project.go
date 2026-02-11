package cli

import (
	"context"
	"fmt"

	"github.com/myuon/track/internal/store/sqlite"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
	}
	cmd.AddCommand(newProjectAddCmd())
	cmd.AddCommand(newProjectListCmd())
	cmd.AddCommand(newProjectShowCmd())
	cmd.AddCommand(newProjectRemoveCmd())
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	var (
		name        string
		description string
	)
	cmd := &cobra.Command{
		Use:   "add <key>",
		Short: "Create a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			if _, err := store.CreateProject(ctx, args[0], name, description); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&description, "description", "", "Project description")
	return cmd
}

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			projects, err := store.ListProjects(ctx)
			if err != nil {
				return err
			}
			for _, p := range projects {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d\n", p.Key, p.Name, p.IssueCount)
			}
			return nil
		},
	}
}

func newProjectShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <key>",
		Short: "Show a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			p, err := store.GetProject(ctx, args[0])
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "key: %s\n", p.Key)
			fmt.Fprintf(cmd.OutOrStdout(), "name: %s\n", p.Name)
			if p.Description != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "description: %s\n", p.Description)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "issue_count: %d\n", p.IssueCount)
			fmt.Fprintf(cmd.OutOrStdout(), "updated_at: %s\n", p.UpdatedAt)
			return nil
		},
	}
}

func newProjectRemoveCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <key>",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			if err := store.DeleteProject(ctx, args[0], force); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Delete linked issue relations too")
	return cmd
}
