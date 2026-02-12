package cli

import (
	"fmt"
	"net/http"

	"github.com/myuon/track/internal/api"
	"github.com/spf13/cobra"
)

var apiListenAndServe = http.ListenAndServe

func newAPICmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Start local API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := fmt.Sprintf("127.0.0.1:%d", port)
			url := fmt.Sprintf("http://%s", addr)
			fmt.Fprintf(cmd.OutOrStdout(), "API running at %s\n", url)
			return apiListenAndServe(addr, api.NewHandler())
		},
	}
	cmd.Flags().IntVar(&port, "port", 8788, "Port")
	return cmd
}
