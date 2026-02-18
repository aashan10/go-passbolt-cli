package tui

import "github.com/passbolt/go-passbolt/api"

// resourceItem implements list.Item for the bubbles list component.
type resourceItem struct {
	id       string
	name     string
	username string
	uri      string
	resource api.Resource
}

func (r resourceItem) Title() string       { return r.name }
func (r resourceItem) Description() string { return r.username + " - " + r.uri }
func (r resourceItem) FilterValue() string { return r.name + " " + r.username + " " + r.uri }

// detailData holds the fetched detail for a single resource.
type detailData struct {
	name         string
	username     string
	uri          string
	password     string
	description  string
	folderID     string
	showPassword bool
}

// Custom tea.Msg types for async operations.

type resourcesLoadedMsg struct {
	items []resourceItem
	err   error
}

type detailLoadedMsg struct {
	data *detailData
	err  error
}

type loginCompleteMsg struct {
	err error
}

type clipboardCopiedMsg struct {
	err error
}
