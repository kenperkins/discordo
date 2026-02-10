package tree

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type Model struct {
	Keybinds Keybinds
	Styles   Styles
	Marker   Marker

	root    *Node
	current *Node

	topLevel     int
	centerCursor bool
	align        bool
	connector    bool
	prefixes     []string

	width  int
	height int
	offset int

	rows []*Node
}

type Marker struct {
	Open  string
	Close string
	Leaf  string
}

func NewModel() Model {
	return Model{
		Keybinds: DefaultKeybinds(),
		Styles:   DefaultStyles(),
		Marker: Marker{
			Open:  "▾ ",
			Close: "▸ ",
			Leaf:  "",
		},
		centerCursor: true,
		connector:    true,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(msg.Width, 0)
		m.height = max(msg.Height, 0)
		m.rebuild()
		return m, nil

	case tea.KeyMsg:
		if len(m.rows) == 0 {
			return m, nil
		}
		k := msg.Key()
		switch {
		case key.Matches(k, m.Keybinds.Up):
			m.move(-1)
		case key.Matches(k, m.Keybinds.Down):
			m.move(1)
		case key.Matches(k, m.Keybinds.Top):
			m.moveToTop()
		case key.Matches(k, m.Keybinds.Bottom):
			m.moveToBottom()
		case key.Matches(k, m.Keybinds.PageUp):
			m.page(-1)
		case key.Matches(k, m.Keybinds.PageDown):
			m.page(1)
		case key.Matches(k, m.Keybinds.Select):
			if m.current == nil {
				return m, nil
			}
			return m, selected(m.current)
		default:
			return m, nil
		}
		m.rebuild()
		return m, nil
	}

	return m, nil
}

func (m Model) View() tea.View {
	if len(m.rows) == 0 {
		return tea.NewView("")
	}

	visible := m.height
	if visible <= 0 {
		// Treat non-positive height as unbounded so the tree still renders before size is wired.
		visible = len(m.rows)
	}
	maxOffset := max(len(m.rows)-visible, 0)
	start := clamp(m.offset, 0, maxOffset)
	if m.height > 0 && m.current != nil {
		if current := m.currentRow(); current >= 0 {
			if current < start {
				start = current
			}
			if current >= start+visible {
				start = current - visible + 1
			}
			start = clamp(start, 0, maxOffset)
		}
	}
	end := min(start+visible, len(m.rows))

	maxDepth := 0
	if !m.connector && m.align {
		maxDepth = m.topLevel
		for _, row := range m.rows {
			maxDepth = max(maxDepth, row.level)
		}
	}

	lines := make([]string, 0, max(m.height, end-start))
	for i := start; i < end; i++ {
		node := m.rows[i]
		isCurrent := node == m.current
		line := m.renderRow(node, maxDepth, isCurrent)
		if m.width > 0 {
			line = ansi.Truncate(line, m.width, "")
			if w := ansi.StringWidth(line); w < m.width {
				pad := strings.Repeat(" ", m.width-w)
				if isCurrent {
					// Style padding too so full-row highlight does not stop at the label width.
					pad = m.selectedRowStyle(node).Render(pad)
				}
				line += pad
			}
		}
		lines = append(lines, line)
	}

	for len(lines) < m.height {
		if m.width > 0 {
			lines = append(lines, strings.Repeat(" ", m.width))
		} else {
			lines = append(lines, "")
		}
	}

	return tea.NewView(strings.Join(lines, "\n"))
}

func (m *Model) SetRoot(root *Node) {
	m.root = root
	m.rebuild()
}

func (m Model) Root() *Node {
	return m.root
}

func (m *Model) SetCurrentNode(node *Node) {
	m.current = node
	m.rebuild()
}

func (m Model) CurrentNode() *Node {
	return m.current
}

func (m Model) VisibleNodes() []*Node {
	return append([]*Node(nil), m.rows...)
}

func (m Model) RowCount() int {
	return len(m.rows)
}

func (m Model) ScrollOffset() int {
	return m.offset
}

func (m *Model) Move(offset int) {
	if offset == 0 {
		return
	}
	m.move(offset)
	m.rebuild()
}

func (m *Model) SetTopLevel(level int) {
	m.topLevel = max(level, 0)
	m.rebuild()
}

func (m *Model) SetCenterCursor(center bool) {
	m.centerCursor = center
	m.rebuild()
}

func (m *Model) SetAlign(align bool) {
	m.align = align
	m.rebuild()
}

func (m *Model) SetConnector(connector bool) {
	m.connector = connector
	m.rebuild()
}

func (m *Model) SetPrefixes(prefixes []string) {
	m.prefixes = prefixes
	m.rebuild()
}

func (m *Model) SetSize(width, height int) {
	m.width = max(width, 0)
	m.height = max(height, 0)
	m.rebuild()
}

func (m Model) Path(node *Node) []*Node {
	if m.root == nil || node == nil {
		return nil
	}

	var path []*Node
	var walk func(cur *Node) bool
	walk = func(cur *Node) bool {
		path = append(path, cur)
		if cur == node {
			return true
		}
		if slices.ContainsFunc(cur.Children(), walk) {
			return true
		}
		path = path[:len(path)-1]
		return false
	}
	if !walk(m.root) {
		return nil
	}
	return path
}

func (m *Model) move(step int) {
	if m.current == nil {
		m.offset += step
		return
	}
	index := m.currentRow()
	if index < 0 {
		return
	}
	m.current = m.findSelectable(index, step)
}

func (m *Model) page(dir int) {
	page := max(m.height, 1)
	if m.current == nil {
		m.offset += dir * page
		return
	}
	index := m.currentRow()
	if index < 0 {
		return
	}
	m.current = m.findSelectable(index, dir*page)
}

func (m *Model) moveToTop() {
	for _, row := range m.rows {
		if row.Selectable() {
			m.current = row
			return
		}
	}
	m.current = nil
}

func (m *Model) moveToBottom() {
	for i := len(m.rows) - 1; i >= 0; i-- {
		if m.rows[i].Selectable() {
			m.current = m.rows[i]
			return
		}
	}
	m.current = nil
}

func (m *Model) currentRow() int {
	for i, row := range m.rows {
		if row == m.current {
			return i
		}
	}
	return -1
}

func (m *Model) findSelectable(start, delta int) *Node {
	step := 1
	if delta < 0 {
		step = -1
	}
	index := start
	remain := delta
	for remain != 0 {
		index += step
		if index < 0 || index >= len(m.rows) {
			// Clamp to the original row to avoid synthetic wraparound when movement overshoots an edge.
			return m.rows[start]
		}
		if m.rows[index].Selectable() {
			remain -= step
		}
	}
	return m.rows[index]
}

func (m *Model) rebuild() {
	m.rows = nil
	if m.root == nil {
		m.current = nil
		m.offset = 0
		return
	}

	type item struct {
		node   *Node
		parent *Node
		level  int
	}

	stack := []item{{
		node:   m.root,
		parent: nil,
		level:  0,
	}}

	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		// Keep traversal metadata in sync with the flattened rows.
		cur.node.parent = cur.parent
		cur.node.level = cur.level

		if cur.level >= m.topLevel {
			m.rows = append(m.rows, cur.node)
		}

		if !cur.node.Expanded() {
			continue
		}
		children := cur.node.Children()
		// Reverse push keeps original child order in pre-order traversal.
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, item{
				node:   children[i],
				parent: cur.node,
				level:  cur.level + 1,
			})
		}
	}

	m.ensureCurrent()
	m.ensureOffset()
}

func (m *Model) ensureCurrent() {
	if len(m.rows) == 0 {
		m.current = nil
		return
	}

	if m.current == nil {
		return
	}

	for _, row := range m.rows {
		if row == m.current && row.Selectable() {
			return
		}
	}
	for n := m.current; n != nil; n = n.parent {
		for _, row := range m.rows {
			if row == n && row.Selectable() {
				// If a selected descendant is hidden by collapse, anchor to the nearest visible selectable ancestor.
				m.current = n
				return
			}
		}
	}

	for _, row := range m.rows {
		if row.Selectable() {
			m.current = row
			return
		}
	}
	m.current = nil
}

func (m *Model) ensureOffset() {
	visible := m.height
	if visible <= 0 {
		visible = len(m.rows)
	}
	maxOffset := max(len(m.rows)-visible, 0)

	if m.current != nil && m.height > 0 {
		current := m.currentRow()
		if current >= 0 {
			if m.centerCursor {
				m.offset = clamp(current-(m.height/2), 0, maxOffset)
			} else {
				if current < m.offset {
					m.offset = current
				}
				if current >= m.offset+m.height {
					m.offset = current - m.height + 1
				}
				m.offset = clamp(m.offset, 0, maxOffset)
			}
			return
		}
	}

	m.offset = clamp(m.offset, 0, maxOffset)
}

func (m Model) renderRow(node *Node, maxDepth int, selected bool) string {
	prefix := m.renderTreePrefix(node, maxDepth)
	marker := m.Marker.Leaf
	if node.Expandable() || len(node.Children()) > 0 {
		// "expandable" shows a closed marker before lazy-loaded children exist, while len(children)>0 keeps loaded nodes togglable.
		if node.Expanded() {
			marker = m.Marker.Open
		} else {
			marker = m.Marker.Close
		}
	}
	labelText := node.Text()
	if len(m.prefixes) > 0 {
		prefixLevel := max(node.level-m.topLevel, 0)
		labelText = m.prefixes[prefixLevel%len(m.prefixes)] + labelText
	}

	connectorStyle := m.Styles.Connector
	markerStyle := m.Styles.Marker
	labelStyle := node.Style().Inherit(m.Styles.Node)
	if selected {
		rowStyle := m.selectedRowStyle(node)
		connectorStyle = connectorStyle.Inherit(rowStyle)
		markerStyle = markerStyle.Inherit(rowStyle)
		labelStyle = labelStyle.Inherit(rowStyle)
	}

	return connectorStyle.Render(prefix) +
		markerStyle.Render(marker) +
		labelStyle.Render(labelText)
}

func (m Model) selectedRowStyle(node *Node) lipgloss.Style {
	style := node.Style().Inherit(m.Styles.Node)
	// Node-selected style should override global selected/default styles.
	style = m.Styles.SelectedNode.Inherit(style)
	style = node.SelectedStyle().Inherit(style)
	return style
}

func (m Model) renderTreePrefix(node *Node, maxDepth int) string {
	if !m.connector {
		depth := max(node.level-m.topLevel, 0)
		if m.align {
			depth = max(maxDepth-m.topLevel, 0)
		}
		return strings.Repeat(" ", depth*max(node.Indent(), 1))
	}

	depth := max(node.level-m.topLevel, 0)
	if depth == 0 {
		return ""
	}
	segmentW := max(node.Indent()+1, 1)

	parts := make([]string, 0, depth)
	for i := 0; i < depth-1; i++ {
		level := m.topLevel + 1 + i
		if hasVerticalAtLevel(node.parent, level, m.topLevel) {
			parts = append(parts, "│"+strings.Repeat(" ", segmentW-1))
		} else {
			parts = append(parts, strings.Repeat(" ", segmentW))
		}
	}
	if isLastChild(node, node.parent) {
		parts = append(parts, "└"+strings.Repeat("─", segmentW-1))
	} else {
		parts = append(parts, "├"+strings.Repeat("─", segmentW-1))
	}

	return strings.Join(parts, "")
}

func clamp(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func isLastChild(node, parent *Node) bool {
	children := parent.Children()
	return children[len(children)-1] == node
}

func hasVerticalAtLevel(ancestor *Node, level, topLevel int) bool {
	for n := ancestor; n != nil; n = n.parent {
		if n.level != level {
			continue
		}
		// Continue vertical line only when ancestor has following siblings.
		parent := n.parent
		if parent.level < topLevel {
			// Stop branch drawing above topLevel so connector lines do not leak into hidden levels.
			return false
		}
		return !isLastChild(n, parent)
	}
	return false
}

var _ help.KeyMap = Model{}

func (m Model) ShortHelp() []key.Binding {
	return []key.Binding{
		m.Keybinds.Up,
		m.Keybinds.Down,
		m.Keybinds.Select,
	}
}

func (m Model) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{m.Keybinds.Up, m.Keybinds.Down, m.Keybinds.Top, m.Keybinds.Bottom},
		{m.Keybinds.PageUp, m.Keybinds.PageDown, m.Keybinds.Select},
	}
}
