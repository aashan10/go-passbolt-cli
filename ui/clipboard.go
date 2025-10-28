package ui

import (
	"fmt"
	"os"
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
	// Check environment first
	display := os.Getenv("DISPLAY")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	sshClient := os.Getenv("SSH_CLIENT")
	
	// Try different clipboard tools in order of preference
	tools := []struct {
		name string
		cmd  []string
		desc string
	}{
		{"xclip", []string{"xclip", "-selection", "clipboard"}, "X11 clipboard"},
		{"xsel", []string{"xsel", "--clipboard", "--input"}, "X11 selection"},
		{"wl-copy", []string{"wl-copy"}, "Wayland clipboard"},
	}

	var detailedErrors []string
	availableTools := []string{}
	
	for _, tool := range tools {
		if _, err := exec.LookPath(tool.cmd[0]); err == nil {
			availableTools = append(availableTools, tool.name)
			cmd := exec.Command(tool.cmd[0], tool.cmd[1:]...)
			cmd.Stdin = strings.NewReader(text)
			
			// Capture both stdout and stderr for debugging
			output, err := cmd.CombinedOutput()
			if err == nil {
				return nil
			} else {
				detailedErrors = append(detailedErrors, fmt.Sprintf("%s: %v (output: %s)", tool.name, err, string(output)))
			}
		}
	}

	// Build comprehensive error message
	var errorParts []string
	
	if len(availableTools) == 0 {
		return fmt.Errorf("no clipboard tools found. Install: xclip, xsel, or wl-copy")
	}
	
	errorParts = append(errorParts, fmt.Sprintf("clipboard failed with tools: %s", strings.Join(availableTools, ", ")))
	
	// Add environment information
	if sshClient != "" {
		errorParts = append(errorParts, "SSH session detected")
		if display == "" {
			errorParts = append(errorParts, "no DISPLAY variable (try: ssh -X)")
		} else {
			errorParts = append(errorParts, fmt.Sprintf("DISPLAY=%s", display))
		}
	}
	
	if display == "" && waylandDisplay == "" {
		errorParts = append(errorParts, "no display server available")
	}
	
	// Add detailed error information
	if len(detailedErrors) > 0 {
		errorParts = append(errorParts, fmt.Sprintf("errors: %s", strings.Join(detailedErrors, "; ")))
	}
	
	return fmt.Errorf("%s", strings.Join(errorParts, ", "))
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