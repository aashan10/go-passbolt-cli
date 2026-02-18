package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/passbolt/go-passbolt-cli/resource"
	"github.com/passbolt/go-passbolt/api"
)

func loadResourcesCmd(sc *sessionClient) tea.Cmd {
	return func() tea.Msg {
		resources, err := sc.client.GetResources(sc.ctx, &api.GetResourcesOptions{
			ContainSecret: false,
		})
		if err != nil {
			return resourcesLoadedMsg{err: err}
		}

		// Decrypt metadata (handles both v4 plaintext and v5 encrypted metadata).
		decrypted, err := resource.DecryptResourcesParallel(sc.ctx, sc.client, resources, false)
		if err != nil {
			return resourcesLoadedMsg{err: err}
		}

		items := make([]resourceItem, 0, len(decrypted))
		for _, d := range decrypted {
			items = append(items, resourceItem{
				id:       d.Resource.ID,
				name:     d.Name,
				username: d.Username,
				uri:      d.URI,
				resource: d.Resource,
			})
		}
		return resourcesLoadedMsg{items: items}
	}
}
