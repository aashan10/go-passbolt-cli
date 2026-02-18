package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newTotpInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "000000"
	ti.CharLimit = 6
	ti.Width = 20
	ti.Focus()
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	return ti
}

func loginCmd(sc *sessionClient) tea.Cmd {
	return func() tea.Msg {
		err := sc.client.Login(sc.ctx)
		return loginCompleteMsg{err: err}
	}
}

func loginUpdate(m model, msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			code := m.totpInput.Value()
			if code == "" {
				return m, nil
			}
			m.session.totpChan <- code
			if !m.loggingIn {
				// First submit - start the login process
				m.loggingIn = true
				m.loginErr = ""
				return m, tea.Batch(m.loginSpinner.Tick, loginCmd(m.session))
			}
			return m, nil
		case "esc", "ctrl+c":
			return m, tea.Quit
		}

	case loginCompleteMsg:
		m.loggingIn = false
		if msg.err != nil {
			m.loginErr = msg.err.Error()
			m.totpInput.Reset()
			m.totpInput.Focus()
			return m, nil
		}
		m.state = stateMain
		m.statusMsg = "Loading resources..."
		m.loading = true
		return m, tea.Batch(m.statusSpinner.Tick, loadResourcesCmd(m.session))

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.loginSpinner, cmd = m.loginSpinner.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.totpInput, cmd = m.totpInput.Update(msg)
	return m, cmd
}

func loginView(m model, width, height int) string {
	boxWidth := 50
	if width < boxWidth+4 {
		boxWidth = width - 4
	}

	title := titleStyle.Render("Passbolt Authentication")

	var content string
	if m.loggingIn {
		content = fmt.Sprintf(
			"%s\n\n  %s Verifying TOTP...\n",
			title,
			m.loginSpinner.View(),
		)
	} else {
		content = fmt.Sprintf(
			"%s\n\n  Enter TOTP code:\n\n  %s\n",
			title,
			m.totpInput.View(),
		)
		if m.loginErr != "" {
			content += "\n  " + errorStyle.Render(m.loginErr) + "\n"
		}
		content += "\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Press Enter to submit, Esc to quit")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(boxWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
