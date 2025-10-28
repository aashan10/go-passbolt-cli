package cmd

import (
	"github.com/passbolt/go-passbolt-cli/ui"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Run a TUI for Passbolt CLI",
	Long:  `Run a TUI for passbolt CLI`,
	RunE:  RunTUI,
}

func RunTUI(cmd *cobra.Command, args []string) error {
	return ui.Run()
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
