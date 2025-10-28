package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
)

func copyToClipboard(text string) error {
	// Try the standard clipboard library first
	err := clipboard.WriteAll(text)
	if err == nil {
		return nil
	}

	// If that fails, try OS-specific fallbacks
	switch runtime.GOOS {
	case "linux":
		return copyToClipboardLinux(text)
	case "darwin":
		return copyToClipboardMacOS(text)
	case "windows":
		return copyToClipboardWindows(text)
	default:
		return fmt.Errorf("clipboard not supported on %s: %w", runtime.GOOS, err)
	}
}

func copyToClipboardLinux(text string) error {
	// Try different clipboard tools in order of preference
	tools := [][]string{
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"wl-copy"}, // Wayland
	}

	for _, tool := range tools {
		if _, err := exec.LookPath(tool[0]); err == nil {
			cmd := exec.Command(tool[0], tool[1:]...)
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
	}

	// If no clipboard tool is available, show instructions
	return fmt.Errorf("no clipboard tool found. Install xclip, xsel, or wl-copy")
}

func copyToClipboardMacOS(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func copyToClipboardWindows(text string) error {
	cmd := exec.Command("clip")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}