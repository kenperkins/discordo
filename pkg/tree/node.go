package tree

import (
	"slices"

	"charm.land/lipgloss/v2"
)

type Node struct {
	reference any
	children  []*Node

	text          string
	style         lipgloss.Style
	selectedStyle lipgloss.Style

	selectable bool
	expanded   bool
	// expandable is separate from children presence so callers can show a closed marker before lazy-loaded children are attached.
	expandable bool
	indent     int

	// Temporary traversal state updated during tree processing.
	parent *Node
	level  int
}

func NewNode(text string) *Node {
	return &Node{
		text:          text,
		style:         lipgloss.NewStyle(),
		selectedStyle: lipgloss.NewStyle().Reverse(true),
		selectable:    true,
		expanded:      true,
		expandable:    false,
		indent:        2,
	}
}

func (n *Node) Walk(callback func(node, parent *Node) bool) *Node {
	stack := []walkItem{{node: n}}
	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !callback(item.node, item.parent) {
			continue
		}
		for i := len(item.node.children) - 1; i >= 0; i-- {
			stack = append(stack, walkItem{
				node:   item.node.children[i],
				parent: item.node,
			})
		}
	}
	return n
}

type walkItem struct {
	node   *Node
	parent *Node
}

func (n *Node) SetReference(reference any) *Node {
	n.reference = reference
	return n
}

func (n *Node) Reference() any {
	return n.reference
}

func (n *Node) SetText(text string) *Node {
	n.text = text
	return n
}

func (n *Node) Text() string {
	return n.text
}

func (n *Node) SetStyle(style lipgloss.Style) *Node {
	n.style = style
	return n
}

func (n *Node) Style() lipgloss.Style {
	return n.style
}

func (n *Node) SetSelectedStyle(style lipgloss.Style) *Node {
	n.selectedStyle = style
	return n
}

func (n *Node) SelectedStyle() lipgloss.Style {
	return n.selectedStyle
}

func (n *Node) SetChildren(children []*Node) *Node {
	n.children = children
	return n
}

func (n *Node) Children() []*Node {
	return n.children
}

func (n *Node) ClearChildren() *Node {
	n.children = nil
	return n
}

func (n *Node) AddChild(node *Node) *Node {
	n.children = append(n.children, node)
	return n
}

func (n *Node) RemoveChild(node *Node) *Node {
	for i, child := range n.children {
		if child == node {
			n.children = slices.Delete(n.children, i, i+1)
			break
		}
	}
	return n
}

func (n *Node) SetSelectable(selectable bool) *Node {
	n.selectable = selectable
	return n
}

func (n *Node) Selectable() bool {
	return n.selectable
}

func (n *Node) SetExpanded(expanded bool) *Node {
	n.expanded = expanded
	return n
}

func (n *Node) SetExpandable(expandable bool) *Node {
	n.expandable = expandable
	return n
}

func (n *Node) Expandable() bool {
	return n.expandable
}

func (n *Node) Expand() *Node {
	n.expanded = true
	return n
}

func (n *Node) Collapse() *Node {
	n.expanded = false
	return n
}

func (n *Node) ExpandAll() *Node {
	n.Walk(func(node, parent *Node) bool {
		node.expanded = true
		return true
	})
	return n
}

func (n *Node) CollapseAll() *Node {
	n.Walk(func(node, parent *Node) bool {
		node.expanded = false
		return true
	})
	return n
}

func (n *Node) Expanded() bool {
	return n.expanded
}

func (n *Node) SetIndent(indent int) *Node {
	n.indent = max(indent, 0)
	return n
}

func (n *Node) Indent() int {
	return n.indent
}
