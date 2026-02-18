package tui

import (
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

type appState int

const (
	stateLogin appState = iota
	stateMain
)

type model struct {
	state  appState
	session *sessionClient
	width  int
	height int

	// Login
	totpInput    textinput.Model
	loginSpinner spinner.Model
	loginErr     string
	loggingIn    bool

	// List pane
	resourceList list.Model
	resources    []resourceItem

	// Detail pane
	detail     *detailData
	detailView viewport.Model

	// Status
	statusMsg      string
	statusSpinner  spinner.Model
	loading        bool

	err error
}

func newModel(sc *sessionClient, initialState appState) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	loginSp := spinner.New()
	loginSp.Spinner = spinner.Dot
	loginSp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("205")).
		BorderForeground(lipgloss.Color("205"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("205")).
		BorderForeground(lipgloss.Color("205"))

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Resources"
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.Styles.Title = titleStyle

	vp := viewport.New(0, 0)

	return model{
		state:         initialState,
		session:       sc,
		totpInput:     newTotpInput(),
		loginSpinner:  loginSp,
		resourceList:  l,
		detailView:    vp,
		statusSpinner: sp,
	}
}

func (m model) Init() tea.Cmd {
	if m.state == stateLogin {
		return tea.Batch(textinput.Blink, m.loginSpinner.Tick)
	}
	// Already logged in - load resources immediately.
	m.loading = true
	return tea.Batch(m.statusSpinner.Tick, loadResourcesCmd(m.session))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		leftWidth := m.width * 2 / 5
		listHeight := m.height - 2
		m.resourceList.SetSize(leftWidth-4, listHeight-2)

		rightWidth := m.width - leftWidth - 3
		topHeight := (m.height - 4) * 7 / 10
		m.detailView.Width = rightWidth - 6
		m.detailView.Height = topHeight - 4

		return m, nil
	}

	if m.state == stateLogin {
		return loginUpdate(m, msg)
	}

	return mainUpdate(m, msg)
}

func mainUpdate(m model, msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys when the list is filtering.
		if m.resourceList.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.resourceList, cmd = m.resourceList.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, keys.Enter):
			if item, ok := m.resourceList.SelectedItem().(resourceItem); ok {
				m.loading = true
				m.statusMsg = "Loading " + item.name + "..."
				return m, tea.Batch(m.statusSpinner.Tick, loadDetailCmd(m.session, item.id, item.name))
			}

		case key.Matches(msg, keys.TogglePass):
			if m.detail != nil {
				m.detail.showPassword = !m.detail.showPassword
				content := renderDetail(m.detail, m.detailView.Width)
				m.detailView.SetContent(content)
			}
			return m, nil

		case key.Matches(msg, keys.CopyPass):
			if m.detail != nil && m.detail.password != "" {
				return m, func() tea.Msg {
					err := clipboard.WriteAll(m.detail.password)
					return clipboardCopiedMsg{err: err}
				}
			}
			m.statusMsg = "No password to copy"
			return m, nil

		case key.Matches(msg, keys.Escape):
			m.detail = nil
			m.statusMsg = ""
			content := renderDetail(nil, m.detailView.Width)
			m.detailView.SetContent(content)
			return m, nil

		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		}

	case resourcesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = formatStatusError(msg.err.Error())
			return m, nil
		}
		items := make([]list.Item, len(msg.items))
		for i, r := range msg.items {
			items[i] = r
		}
		m.resourceList.SetItems(items)
		m.resources = msg.items
		m.statusMsg = formatStatusSuccess(fmt.Sprintf("Loaded %d resources", len(items)))
		return m, nil

	case detailLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = formatStatusError(msg.err.Error())
			return m, nil
		}
		m.detail = msg.data
		content := renderDetail(m.detail, m.detailView.Width)
		m.detailView.SetContent(content)
		m.statusMsg = formatStatusSuccess("Details loaded")
		return m, nil

	case clipboardCopiedMsg:
		if msg.err != nil {
			m.statusMsg = formatStatusError("Clipboard: " + msg.err.Error())
		} else {
			m.statusMsg = formatStatusSuccess("Password copied to clipboard!")
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.statusSpinner, cmd = m.statusSpinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// Forward remaining messages to the list component.
	var cmd tea.Cmd
	m.resourceList, cmd = m.resourceList.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.state == stateLogin {
		return loginView(m, m.width, m.height)
	}

	leftWidth := m.width * 2 / 5
	rightWidth := m.width - leftWidth - 1
	totalHeight := m.height
	topHeight := (totalHeight - 4) * 7 / 10
	bottomHeight := totalHeight - topHeight - 4

	// Left pane: resource list
	leftPane := listPaneStyle.
		Width(leftWidth - 2).
		Height(totalHeight - 2).
		Render(m.resourceList.View())

	// Right top: detail
	detailContent := m.detailView.View()
	detailPane := detailPaneStyle.
		Width(rightWidth - 2).
		Height(topHeight).
		Render(detailContent)

	// Right bottom: status
	statusContent := renderStatus(m.statusMsg, m.loading, m.statusSpinner, rightWidth-6)
	statusPane := statusPaneStyle.
		Width(rightWidth - 2).
		Height(bottomHeight).
		Render(statusContent)

	rightPane := lipgloss.JoinVertical(lipgloss.Left, detailPane, statusPane)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

// Run is the entry point called by the cobra command.
func Run(cmd *cobra.Command, args []string) error {
	sc, err := newSessionClient()
	if err != nil {
		return err
	}

	initialState := stateMain
	if sc.needMFA {
		initialState = stateLogin
	}

	m := newModel(sc, initialState)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		sc.close()
		return err
	}

	final := finalModel.(model)
	final.session.close()

	if final.err != nil {
		return final.err
	}
	return nil
}
