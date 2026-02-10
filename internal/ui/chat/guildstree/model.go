package guildstree

import (
	"cmp"
	"fmt"
	"image/color"
	"log/slog"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ayn2op/discordo/internal/config"
	"github.com/ayn2op/discordo/pkg/tree"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/ningen/v3"
)

type dmNode struct{}

type Model struct {
	cfg   *config.Config
	state *ningen.State
	tree  tree.Model
}

func NewModel(cfg *config.Config, state *ningen.State) Model {
	treeModel := tree.NewModel()
	treeModel.SetTopLevel(1)

	gt := cfg.Keybinds.GuildsTree
	treeModel.Keybinds = tree.Keybinds{
		Up:       gt.Up.Binding,
		Down:     gt.Down.Binding,
		Top:      gt.Top.Binding,
		Bottom:   gt.Bottom.Binding,
		PageUp:   gt.PageUp.Binding,
		PageDown: gt.PageDown.Binding,

		Select: gt.Select.Binding,
	}
	return Model{
		cfg:   cfg,
		state: state,
		tree:  treeModel,
	}
}

func (m Model) Init() tea.Cmd {
	return m.tree.Init()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k := msg.Key()
		kbs := m.cfg.Keybinds
		switch {
		case key.Matches(k, kbs.GuildsTree.YankID.Binding):
			if node := m.tree.CurrentNode(); node != nil {
				if id, ok := node.Reference().(fmt.Stringer); ok {
					return m, tea.SetClipboard(id.String())
				}
			}
		}

	case tree.SelectedMsg:
		m.onSelected(msg.Node)
		return m, nil

	case *gateway.ReadyEvent:
		dmNode := tree.NewNode("Direct Messages").
			SetReference(dmNode{}).
			SetExpandable(true).
			SetExpanded(false)
		root := tree.NewNode("").AddChild(dmNode)

		// Build index of all available guilds.
		guildsByID := make(map[discord.GuildID]*gateway.GuildCreateEvent, len(msg.Guilds))
		for i := range msg.Guilds {
			guildsByID[msg.Guilds[i].ID] = &msg.Guilds[i]
		}

		// Track guilds already in folders to identify orphans.
		guildsInFolders := make(map[discord.GuildID]bool)
		for _, folder := range msg.UserSettings.GuildFolders {
			for _, guildID := range folder.GuildIDs {
				guildsInFolders[guildID] = true
			}
		}

		// Use GuildPositions as canonical order.
		positions := msg.UserSettings.GuildPositions
		if len(positions) == 0 {
			positions = make([]discord.GuildID, 0, len(msg.Guilds))
			for _, guildEvent := range msg.Guilds {
				positions = append(positions, guildEvent.ID)
			}
		}

		// Add orphan guilds (guilds not in folders) directly to root.
		for _, guildID := range positions {
			if guildsInFolders[guildID] {
				continue
			}
			guild, ok := guildsByID[guildID]
			if !ok {
				continue
			}
			guildNode := tree.NewNode(guild.Name).
				SetReference(guild.ID).
				SetExpandable(true).
				SetExpanded(false)
			root.AddChild(guildNode)
		}

		// Process folders (including single-guild pseudo-folders).
		for _, folder := range msg.UserSettings.GuildFolders {
			if folder.ID == 0 && len(folder.GuildIDs) == 1 {
				// Discord uses this shape for non-foldered guild entries that still appear in GuildFolders.
				guild, ok := guildsByID[folder.GuildIDs[0]]
				if !ok {
					continue
				}
				guildNode := tree.NewNode(guild.Name).
					SetReference(guild.ID).
					SetExpandable(true).
					SetExpanded(false)
				root.AddChild(guildNode)
				continue
			}

			name := "Folder"
			if folder.Name != "" {
				name = folder.Name
			}
			folderNode := tree.NewNode(name)
			if folder.Color > 0 {
				r, g, b := folder.Color.RGB()
				folderNode.SetStyle(lipgloss.NewStyle().Foreground(color.RGBA{
					R: r,
					G: g,
					B: b,
					A: 0xFF,
				}))
			}
			for _, guildID := range folder.GuildIDs {
				guild, ok := guildsByID[guildID]
				if !ok {
					continue
				}
				folderNode.AddChild(tree.NewNode(guild.Name).SetReference(guild.ID).SetExpandable(true).SetExpanded(false))
			}
			root.AddChild(folderNode)
		}

		m.tree.SetRoot(root)
		m.tree.SetCurrentNode(root)
	}

	var cmd tea.Cmd
	m.tree, cmd = m.tree.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	return m.tree.View()
}

func (m *Model) onSelected(node *tree.Node) {
	// Loaded nodes toggle immediately; no fetch needed.
	if len(node.Children()) > 0 {
		node.SetExpanded(!node.Expanded())
		m.tree.SetCurrentNode(node)
		return
	}
	// Expandable+expanded with zero children can happen before/after lazy-load paths; collapse should still work.
	if node.Expandable() && node.Expanded() {
		node.Collapse()
		m.tree.SetCurrentNode(node)
		return
	}

	switch ref := node.Reference().(type) {
	case discord.GuildID:
		// Guild channels are lazy-loaded on first selection to avoid work at startup.
		channels, err := m.state.Cabinet.Channels(ref)
		if err != nil {
			slog.Error("failed to get channels", "err", err, "guild_id", ref)
			return
		}
		sortGuildChannels(channels)
		m.createChannelNodes(node, channels)
		node.Expand()
		m.tree.SetCurrentNode(node)

	case discord.ChannelID:
		channel, err := m.state.Cabinet.Channel(ref)
		if err != nil {
			slog.Error("failed to get channel from state", "channel_id", ref, "err", err)
			return
		}

		if channel.Type == discord.GuildForum {
			allChannels, err := m.state.Cabinet.Channels(channel.GuildID)
			if err != nil {
				slog.Error("failed to get channels for forum threads", "err", err, "guild_id", channel.GuildID)
				return
			}
			for _, ch := range allChannels {
				if ch.ParentID == channel.ID && (ch.Type == discord.GuildPublicThread ||
					ch.Type == discord.GuildPrivateThread ||
					ch.Type == discord.GuildAnnouncementThread) {
					m.createChannelNode(node, ch)
				}
			}
			node.Expand()
			m.tree.SetCurrentNode(node)
		}

	case dmNode:
		channels, err := m.state.PrivateChannels()
		if err != nil {
			slog.Error("failed to get private channels", "err", err)
			return
		}
		sortPrivateChannels(channels)
		for _, channel := range channels {
			m.createChannelNode(node, channel)
		}
		node.Expand()
		m.tree.SetCurrentNode(node)
	}
}

func (m *Model) createChannelNodes(parent *tree.Node, channels []discord.Channel) {
	hasChildByParentID := make(map[discord.ChannelID]struct{}, len(channels))
	for _, channel := range channels {
		if channel.ParentID.IsValid() {
			hasChildByParentID[channel.ParentID] = struct{}{}
		}
	}

	for _, channel := range channels {
		if channel.Type != discord.GuildCategory && !channel.ParentID.IsValid() {
			// Keep top-level non-category channels before categories to match Discord's visual grouping.
			m.createChannelNode(parent, channel)
		}
	}

	// Add only categories that have at least one child channel.
	for _, channel := range channels {
		if channel.Type == discord.GuildCategory {
			if _, ok := hasChildByParentID[channel.ID]; ok {
				m.createChannelNode(parent, channel)
			}
		}
	}

	for _, channel := range channels {
		if !channel.ParentID.IsValid() {
			continue
		}
		var categoryNode *tree.Node
		for _, childNode := range parent.Children() {
			id, ok := childNode.Reference().(discord.ChannelID)
			if ok && id == channel.ParentID {
				categoryNode = childNode
				break
			}
		}
		if categoryNode != nil {
			m.createChannelNode(categoryNode, channel)
		}
	}
}

func (m *Model) createChannelNode(parent *tree.Node, channel discord.Channel) {
	if channel.Type != discord.DirectMessage && channel.Type != discord.GroupDM && !m.state.HasPermissions(channel.ID, discord.PermissionViewChannel) {
		return
	}
	node := tree.NewNode(channelToString(channel, m.cfg.Icons)).SetReference(channel.ID)
	if channel.Type == discord.GuildForum {
		node.SetExpandable(true).SetExpanded(false)
	}
	parent.AddChild(node)
}

func sortGuildChannels(channels []discord.Channel) {
	slices.SortFunc(channels, func(a, b discord.Channel) int {
		return cmp.Compare(a.Position, b.Position)
	})
}

func sortPrivateChannels(channels []discord.Channel) {
	slices.SortFunc(channels, func(a, b discord.Channel) int {
		return cmp.Compare(lastMessageID(b), lastMessageID(a))
	})
}

func lastMessageID(channel discord.Channel) discord.MessageID {
	if channel.LastMessageID.IsValid() {
		return channel.LastMessageID
	}
	return discord.MessageID(channel.ID)
}

func channelToString(channel discord.Channel, icons config.Icons) string {
	var icon string
	switch channel.Type {
	case discord.DirectMessage, discord.GroupDM:
		if channel.Name != "" {
			return channel.Name
		}

		recipients := make([]string, len(channel.DMRecipients))
		for i, r := range channel.DMRecipients {
			recipients[i] = r.DisplayOrUsername()
		}
		return strings.Join(recipients, ", ")

	case discord.GuildCategory:
		icon = icons.GuildCategory
	case discord.GuildText:
		icon = icons.GuildText
	case discord.GuildVoice:
		icon = icons.GuildVoice
	case discord.GuildStageVoice:
		icon = icons.GuildStageVoice
	case discord.GuildAnnouncementThread:
		icon = icons.GuildAnnouncementThread
	case discord.GuildPublicThread:
		icon = icons.GuildPublicThread
	case discord.GuildPrivateThread:
		icon = icons.GuildPrivateThread
	case discord.GuildAnnouncement:
		icon = icons.GuildAnnouncement
	case discord.GuildForum:
		icon = icons.GuildForum
	case discord.GuildStore:
		icon = icons.GuildStore
	}

	return icon + channel.Name
}
