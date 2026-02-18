package cmd

import (
	"github.com/passbolt/go-passbolt-cli/tui"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Launch interactive TUI for browsing Passbolt resources",
	Long:  `Launch an interactive terminal user interface for browsing and managing Passbolt resources.`,
	RunE:  tui.Run,
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
