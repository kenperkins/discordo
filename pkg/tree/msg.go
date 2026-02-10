package tree

import tea "charm.land/bubbletea/v2"

type SelectedMsg struct {
	Node *Node
}

func selected(node *Node) tea.Cmd {
	return func() tea.Msg {
		return SelectedMsg{Node: node}
	}
}
