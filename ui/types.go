package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/passbolt/go-passbolt/api"
)

type Resource struct {
	ID          string
	Name        string
	Username    string
	URI         string
	Password    string
	Description string
	FolderID    string
	Loaded      bool
}

type Model struct {
	resources     []Resource
	filtered      []Resource
	selected      int
	filtering     bool
	filterText    string
	width         int
	height        int
	leftPaneWidth int
	viewportTop   int
	viewportSize  int
	err           error
	loading       bool
	loadingDetail bool
	showHelp      bool
	clipboardMsg  string
	client        *api.Client
	ctx           context.Context
}

type resourcesLoadedMsg []Resource
type resourceDetailLoadedMsg Resource
type errorMsg struct{ err error }
type clipboardClearMsg struct{}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		loadResources,
	)
}