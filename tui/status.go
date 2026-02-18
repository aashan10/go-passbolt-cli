package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

const helpText = "j/k: navigate | Enter: details | t: toggle password | y: copy password | /: filter | q: quit"

func renderStatus(statusMsg string, loading bool, sp spinner.Model, width int) string {
	var content string
	if loading {
		content = sp.View() + " " + statusMsg
	} else if statusMsg != "" {
		content = statusMsg
	}

	help := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(helpText)

	if content != "" {
		// Wrap long content
		if len(content) > width-4 {
			content = content[:width-4]
		}
		return fmt.Sprintf("%s\n\n%s", content, help)
	}
	return help
}

func formatStatusError(msg string) string {
	return errorStyle.Render("Error: " + msg)
}

func formatStatusSuccess(msg string) string {
	return successStyle.Render(msg)
}

func wrapText(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}
	var lines []string
	for len(text) > width {
		lines = append(lines, text[:width])
		text = text[width:]
	}
	if len(text) > 0 {
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}
