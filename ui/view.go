package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress q to quit.", m.err)
	}

	// Handle help popup
	if m.showHelp {
		helpContent := `Passbolt CLI TUI - Key Bindings

Navigation:
  ↑/k         Move up
  ↓/j         Move down
  Enter       Load resource details

Actions:
  u           Copy username to clipboard
  p           Copy password to clipboard
  /           Start filtering resources
  ?           Toggle this help
  q/Ctrl+C    Quit

Filtering:
  Enter       Apply filter
  Esc         Cancel filter
  Backspace   Delete character

Press ? again to close this help.`

		help := HelpStyle.Width(60).Height(20).Render(helpContent)
		
		// Center the help dialog
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, help)
	}

	// Calculate dimensions
	if m.width == 0 {
		m.width = 80
		m.height = 24
	}
	m.leftPaneWidth = m.width / 2
	
	// Update viewport size based on actual dimensions
	m.viewportSize = m.height - 10
	if m.viewportSize <= 0 {
		m.viewportSize = 10
	}

	// Build left pane (resource list)
	var leftPaneContent string
	
	title := TitleStyle.Render("Passbolt Resources")
	if m.filtering {
		title += " " + FilterStyle.Render(fmt.Sprintf("Filter: %s", m.filterText))
	}
	
	// Add scroll indicator
	scrollInfo := ""
	if len(m.filtered) > m.viewportSize {
		scrollInfo = fmt.Sprintf(" (%d/%d)", m.selected+1, len(m.filtered))
	}
	title += scrollInfo
	
	leftPaneContent = title + "\n\n"

	if m.loading {
		leftPaneContent += "Loading resources...\n"
	} else if len(m.filtered) == 0 && !m.loading {
		if m.filtering && m.filterText != "" {
			leftPaneContent += "No resources match filter\n"
		} else {
			leftPaneContent += "No resources found\n"
		}
	} else {
		// Calculate visible range
		start := m.viewportTop
		end := start + m.viewportSize
		if end > len(m.filtered) {
			end = len(m.filtered)
		}
		
		// Render only visible items
		for i := start; i < end; i++ {
			res := m.filtered[i]
			if i == m.selected {
				leftPaneContent += SelectedItemStyle.Render(fmt.Sprintf("► %s ◄", res.Name)) + "\n"
			} else {
				leftPaneContent += ItemStyle.Render(fmt.Sprintf("  %s", res.Name)) + "\n"
			}
		}
		
		// Add scroll indicators
		if m.viewportTop > 0 {
			leftPaneContent = "  ▲ More above\n" + leftPaneContent
		}
		if end < len(m.filtered) {
			leftPaneContent += "  ▼ More below\n"
		}
	}

	leftPane := LeftPaneStyle.
		Width(m.leftPaneWidth - 4).
		Height(m.height - 6).
		Render(leftPaneContent)

	// Build right pane (details)
	var rightPaneContent string
	
	if len(m.filtered) > 0 && m.selected < len(m.filtered) && !m.loading {
		selected := m.filtered[m.selected]
		if selected.Loaded {
			// Build details with conditional password/username display
			username := selected.Username
			password := "***hidden***"
			
			if m.showUsername {
				username = fmt.Sprintf("%s (VISIBLE - clipboard failed)", selected.Username)
			}
			if m.showPassword {
				password = fmt.Sprintf("%s (VISIBLE - clipboard failed)", selected.Password)
			}
			
			rightPaneContent = fmt.Sprintf("Name: %s\n\nUsername: %s\n\nURI: %s\n\nPassword: %s\n\nDescription: %s",
				selected.Name,
				username,
				selected.URI,
				password,
				selected.Description)
		} else if m.loadingDetail {
			rightPaneContent = "Loading details..."
		} else {
			rightPaneContent = fmt.Sprintf("Resource: %s\n\nPress Enter to load details\n\nThis will fetch the username, password, and other details from the server.", selected.Name)
		}
	} else {
		rightPaneContent = "Resource Details\n\nSelect a resource from the left pane and press Enter to view its details."
	}

	rightPane := DetailStyle.
		Width(m.width - m.leftPaneWidth - 6).
		Height(m.height - 6).
		Render(rightPaneContent)

	// Build status bar
	var statusBar string
	if m.clipboardMsg != "" {
		if strings.HasPrefix(m.clipboardMsg, "✓") {
			statusBar = ClipboardStyle.Render(m.clipboardMsg)
		} else {
			statusBar = ErrorStyle.Render(m.clipboardMsg)
		}
	} else if m.filtering {
		statusBar = StatusStyle.Render("Enter: apply filter, Esc: cancel")
	} else if m.loading {
		statusBar = LoadingStyle.Render("● Loading resources...")
	} else if m.loadingDetail {
		statusBar = LoadingStyle.Render("● Loading details...")
	} else {
		statusBar = StatusStyle.Render("↑/↓: navigate, Enter: load details, u: copy username, p: copy password, ?: help, q: quit")
	}

	// Combine everything
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPane,
		rightPane,
	)

	// Add padding at the top
	paddingLines := "\n\n\n"
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		paddingLines,
		content,
		"\n"+statusBar,
	)
}