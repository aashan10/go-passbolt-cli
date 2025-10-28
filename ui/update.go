package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/passbolt/go-passbolt-cli/util"
	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
)

func loadResources() tea.Msg {
	ctx := util.GetContext()
	client, err := util.GetClient(ctx)
	if err != nil {
		return errorMsg{err}
	}

	resources, err := client.GetResources(ctx, &api.GetResourcesOptions{})
	if err != nil {
		client.Logout(context.TODO())
		return errorMsg{fmt.Errorf("listing resources: %w", err)}
	}

	var result []Resource
	for _, res := range resources {
		result = append(result, Resource{
			ID:       res.ID,
			Name:     res.Name,
			FolderID: res.FolderParentID,
			Loaded:   false,
		})
	}

	return resourcesLoadedMsg(result)
}

func (m Model) loadResourceDetail(id string) tea.Cmd {
	return func() tea.Msg {
		_, name, username, uri, password, description, err := helper.GetResource(m.ctx, m.client, id)
		if err != nil {
			return errorMsg{fmt.Errorf("getting resource detail: %w", err)}
		}

		return resourceDetailLoadedMsg(Resource{
			ID:          id,
			Name:        name,
			Username:    username,
			URI:         uri,
			Password:    password,
			Description: description,
			Loaded:      true,
		})
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.leftPaneWidth = m.width / 2
		// Calculate viewport size (height minus header, borders, and status bar)
		m.viewportSize = m.height - 10

	case resourcesLoadedMsg:
		m.resources = []Resource(msg)
		m.filtered = m.resources
		m.loading = false
		
		// Initialize client and context for detail loading
		ctx := util.GetContext()
		client, err := util.GetClient(ctx)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.client = client
		m.ctx = ctx
		
		// Don't auto-load details anymore

	case resourceDetailLoadedMsg:
		m.loadingDetail = false
		resource := Resource(msg)
		// Update the resource in both resources and filtered arrays
		for i := range m.resources {
			if m.resources[i].ID == resource.ID {
				m.resources[i] = resource
				break
			}
		}
		for i := range m.filtered {
			if m.filtered[i].ID == resource.ID {
				m.filtered[i] = resource
				break
			}
		}

	case errorMsg:
		m.err = msg.err
		m.loading = false
		m.loadingDetail = false

	case clipboardClearMsg:
		m.clipboardMsg = ""

	case tea.KeyMsg:
		if m.filtering {
			switch msg.Type {
			case tea.KeyEnter:
				m.filtering = false
				m.applyFilter()
			case tea.KeyEsc:
				m.filtering = false
				m.filterText = ""
				m.filtered = m.resources
			case tea.KeyBackspace:
				if len(m.filterText) > 0 {
					m.filterText = m.filterText[:len(m.filterText)-1]
					m.applyFilter()
				}
			case tea.KeyRunes:
				m.filterText += string(msg.Runes)
				m.applyFilter()
			}
		} else {
			switch msg.Type {
			case tea.KeyCtrlC:
				if m.client != nil {
					m.client.Logout(context.TODO())
				}
				return m, tea.Quit
			case tea.KeyUp:
				if m.selected > 0 {
					m.selected--
					m.adjustViewport()
				}
			case tea.KeyDown:
				if m.selected < len(m.filtered)-1 {
					m.selected++
					m.adjustViewport()
				}
			case tea.KeyEnter:
				// Load details when Enter is pressed
				if len(m.filtered) > 0 && !m.filtered[m.selected].Loaded && !m.loadingDetail {
					m.loadingDetail = true
					return m, m.loadResourceDetail(m.filtered[m.selected].ID)
				}
			case tea.KeyRunes:
				switch string(msg.Runes) {
				case "q":
					if m.client != nil {
						m.client.Logout(context.TODO())
					}
					return m, tea.Quit
				case "?":
					m.showHelp = !m.showHelp
				case "k":
					if m.selected > 0 {
						m.selected--
						m.adjustViewport()
					}
				case "j":
					if m.selected < len(m.filtered)-1 {
						m.selected++
						m.adjustViewport()
					}
				case "/":
					m.filtering = true
					m.filterText = ""
				case "u":
					if len(m.filtered) > 0 && m.filtered[m.selected].Loaded {
						if err := copyToClipboard(m.filtered[m.selected].Username); err == nil {
							m.clipboardMsg = "✓ Username copied to clipboard!"
							return m, tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
								return clipboardClearMsg{}
							})
						} else {
							// Fallback: show username in the detail pane
							m.clipboardMsg = "⚠ Clipboard failed - Username shown in details"
							return m, tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
								return clipboardClearMsg{}
							})
						}
					}
				case "p":
					if len(m.filtered) > 0 && m.filtered[m.selected].Loaded {
						if err := copyToClipboard(m.filtered[m.selected].Password); err == nil {
							m.clipboardMsg = "✓ Password copied to clipboard!"
							return m, tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
								return clipboardClearMsg{}
							})
						} else {
							// Fallback: show password in the detail pane  
							m.clipboardMsg = "⚠ Clipboard failed - Password shown in details"
							return m, tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
								return clipboardClearMsg{}
							})
						}
					}
				}
			}
		}
	}

	return m, nil
}

func (m *Model) applyFilter() {
	if m.filterText == "" {
		m.filtered = m.resources
		m.selected = 0
		m.viewportTop = 0
		return
	}

	m.filtered = []Resource{}
	filter := strings.ToLower(m.filterText)
	for _, res := range m.resources {
		if strings.Contains(strings.ToLower(res.Name), filter) ||
			strings.Contains(strings.ToLower(res.Username), filter) ||
			strings.Contains(strings.ToLower(res.URI), filter) {
			m.filtered = append(m.filtered, res)
		}
	}
	m.selected = 0
	m.viewportTop = 0
}

func (m *Model) adjustViewport() {
	if m.viewportSize <= 0 {
		m.viewportSize = 10 // default fallback
	}

	// Ensure selected item is visible in viewport
	if m.selected < m.viewportTop {
		// Selected item is above viewport, scroll up
		m.viewportTop = m.selected
	} else if m.selected >= m.viewportTop+m.viewportSize {
		// Selected item is below viewport, scroll down
		m.viewportTop = m.selected - m.viewportSize + 1
	}

	// Ensure viewport doesn't go beyond bounds
	if m.viewportTop < 0 {
		m.viewportTop = 0
	}
	
	maxTop := len(m.filtered) - m.viewportSize
	if maxTop < 0 {
		maxTop = 0
	}
	if m.viewportTop > maxTop {
		m.viewportTop = maxTop
	}
}