package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/passbolt/go-passbolt/helper"
)

func loadDetailCmd(sc *sessionClient, resourceID, resourceName string) tea.Cmd {
	return func() tea.Msg {
		folderParentID, name, username, uri, password, description, err := helper.GetResource(
			sc.ctx,
			sc.client,
			resourceID,
		)
		if err != nil {
			return detailLoadedMsg{err: err}
		}
		_ = resourceName
		return detailLoadedMsg{
			data: &detailData{
				name:         name,
				username:     username,
				uri:          uri,
				password:     password,
				description:  description,
				folderID:     folderParentID,
				showPassword: false,
			},
		}
	}
}

func renderDetail(d *detailData, width int) string {
	if d == nil {
		return hiddenStyle.Render("Press Enter on a resource to view details")
	}

	passwordDisplay := hiddenStyle.Render(strings.Repeat("*", 12))
	if d.showPassword {
		passwordDisplay = d.password
	}

	fieldWidth := width - 16 // account for label width + padding
	if fieldWidth < 20 {
		fieldWidth = 20
	}

	lines := []string{
		fmt.Sprintf("%s  %s", labelStyle.Render("Name:       "), wrapText(d.name, fieldWidth)),
		fmt.Sprintf("%s  %s", labelStyle.Render("Username:   "), wrapText(d.username, fieldWidth)),
		fmt.Sprintf("%s  %s", labelStyle.Render("URI:        "), wrapText(d.uri, fieldWidth)),
		fmt.Sprintf("%s  %s", labelStyle.Render("Password:   "), passwordDisplay),
		fmt.Sprintf("%s  %s", labelStyle.Render("Description:"), wrapText(d.description, fieldWidth)),
	}

	if d.folderID != "" {
		lines = append(lines,
			fmt.Sprintf("%s  %s", labelStyle.Render("Folder:     "), d.folderID),
		)
	}

	return strings.Join(lines, "\n")
}
