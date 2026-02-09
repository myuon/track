package cli

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"

	appconfig "github.com/myuon/track/internal/config"
	"github.com/myuon/track/internal/ui"
	"github.com/spf13/cobra"
)

func newUICmd() *cobra.Command {
	var port int
	var open bool

	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Start local web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := appconfig.Load()
			if err != nil {
				return err
			}
			if port == 0 {
				port = cfg.UIPort
			}

			url := fmt.Sprintf("http://127.0.0.1:%d", port)
			if open || cfg.OpenBrowser {
				_ = openBrowser(url)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "UI running at %s\n", url)
			return http.ListenAndServe(fmt.Sprintf(":%d", port), ui.NewHandler())
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "Port (default from config or 8787)")
	cmd.Flags().BoolVar(&open, "open", false, "Open browser")
	return cmd
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported OS for --open")
	}
}
