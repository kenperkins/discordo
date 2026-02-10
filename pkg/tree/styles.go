package tree

import "charm.land/lipgloss/v2"

type Styles struct {
	Node         lipgloss.Style
	SelectedNode lipgloss.Style

	Connector lipgloss.Style
	Marker    lipgloss.Style
}

func DefaultStyles() Styles {
	return Styles{
		Node:         lipgloss.NewStyle(),
		SelectedNode: lipgloss.NewStyle().Reverse(true),

		Connector: lipgloss.NewStyle().Faint(true),
		Marker:    lipgloss.NewStyle(),
	}
}
