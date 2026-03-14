// Package ui provides the terminal user interface for the Meshtastic messaging application.
package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"meshtastic_go/internal/transport"
	"meshtastic_go/internal/ui/store"
	"meshtastic_go/pkg/generated"
)

// Layout constants.
const (
	defaultWidth  = 120
	defaultHeight = 40
	statusBarH    = 1
	inputAreaH    = 3
	helpBarH      = 1
	sidebarW      = 32
)

// ---------- tea.Msg types ----------

// radioEventMsg wraps a single RadioEvent received from the client.
type radioEventMsg struct{ event transport.RadioEvent }

// viewMode distinguishes channel view from DM view.
type viewMode int

const (
	viewChannel viewMode = iota
	viewDM
)

// focusArea tracks which panel owns keyboard input.
type focusArea int

const (
	focusInput   focusArea = iota // text input: Enter sends, arrows scroll viewport
	focusSidebar                  // sidebar: Enter selects, arrows navigate list
)

// ---------- Key bindings ----------

type keyMap struct {
	Quit     key.Binding
	Send     key.Binding
	NextChan key.Binding
	PrevChan key.Binding
	Tab      key.Binding
	Esc      key.Binding
}

var keys = keyMap{
	Quit:     key.NewBinding(key.WithKeys("ctrl+c")),
	Send:     key.NewBinding(key.WithKeys("enter")),
	NextChan: key.NewBinding(key.WithKeys("ctrl+n")),
	PrevChan: key.NewBinding(key.WithKeys("ctrl+p")),
	Tab:      key.NewBinding(key.WithKeys("tab")),
	Esc:      key.NewBinding(key.WithKeys("esc")),
}

// ---------- Styles ----------

var (
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("236")).
			Bold(true)

	sidebarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderRight(true).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	sidebarTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	sidebarItemActive = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Bold(true)

	sidebarItem = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	sidebarScrollHint = lipgloss.NewStyle().
				Foreground(lipgloss.Color("243")).
				Italic(true)

	helpBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Background(lipgloss.Color("238"))

	helpKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("238")).
				Bold(true)

	helpDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250")).
				Background(lipgloss.Color("238"))

	helpSepStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("243")).
				Background(lipgloss.Color("238"))

	incomingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	outgoingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	systemStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
	timeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Width(9)
	nameStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).Width(16)
)

// ---------- Model ----------

// Model is the Bubble Tea model for the Meshtastic TUI.
type Model struct {
	client *transport.Client

	width, height int
	ready         bool

	// Messaging state
	messageStore *store.MessageStore
	mode         viewMode
	channelIdx   int    // index into the channels slice for the active channel
	dmTarget     uint32 // node num of DM target (when mode == viewDM)

	// Focus & sidebar state
	focus         focusArea // which panel owns input
	sidebarTab    int       // 0 = channels, 1 = nodes
	sidebarCursor int
	sidebarOffset int // first visible item index (scroll offset)

	// Cached data from client.State (refreshed on events)
	channels []*generated.Channel
	nodes    []*generated.NodeInfo

	// UI components
	viewport  viewport.Model
	textInput textinput.Model

	connStatus string
}

// New creates a Model wired to the given connected Client.
func New(client *transport.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message…"
	ti.Focus()
	ti.CharLimit = 228 // Meshtastic max payload ~237 bytes; leave headroom

	vp := viewport.New(defaultWidth-sidebarW, defaultHeight-statusBarH-inputAreaH-helpBarH)

	ms := store.NewMessageStore()
	ms.CreateSystemMessage("Welcome to Meshtastic Go!")

	// Snapshot initial state — filter out disabled channels.
	channels := filterActiveChannels(client.State.Channels())
	nodes := client.State.Nodes()
	sortNodes(nodes)

	return Model{
		client:       client,
		width:        defaultWidth,
		height:       defaultHeight,
		messageStore: ms,
		mode:         viewChannel,
		channels:     channels,
		nodes:        nodes,
		viewport:     vp,
		textInput:    ti,
		connStatus:   "Connected",
	}
}

// Init returns the initial command — start listening for radio events.
func (m Model) Init() tea.Cmd {
	return m.listenForEvent()
}

// listenForEvent returns a tea.Cmd that blocks on client.Events and
// wraps the next event as a radioEventMsg.
func (m Model) listenForEvent() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-m.client.Events
		if !ok {
			// Channel closed — radio disconnected.
			return radioEventMsg{event: transport.ConnectionEvent{
				Status:  transport.StatusDisconnected,
				Message: "Radio connection closed",
			}}
		}
		return radioEventMsg{event: ev}
	}
}

// ---------- Update ----------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = m.width - sidebarW
		m.viewport.Height = m.height - statusBarH - inputAreaH - helpBarH
		m.textInput.Width = m.width - sidebarW - 4
		m.ready = true
		m.refreshMessages()

	case radioEventMsg:
		m.handleRadioEvent(msg.event)
		m.refreshMessages()
		cmds = append(cmds, m.listenForEvent())

	case tea.KeyMsg:
		cmd := m.handleKey(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Only forward key events to the component that has focus.
		if m.focus == focusInput {
			var tiCmd tea.Cmd
			m.textInput, tiCmd = m.textInput.Update(msg)
			cmds = append(cmds, tiCmd)
		}
		// Viewport scrolls only when input is focused (not when sidebar navigates).
		if m.focus == focusInput {
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			cmds = append(cmds, vpCmd)
		}
		return m, tea.Batch(cmds...)

	default:
		// Non-key messages (window size, etc.) go to all components.
		var tiCmd tea.Cmd
		m.textInput, tiCmd = m.textInput.Update(msg)
		cmds = append(cmds, tiCmd)
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}

	return m, tea.Batch(cmds...)
}

// handleRadioEvent processes a single RadioEvent and updates model state.
func (m *Model) handleRadioEvent(ev transport.RadioEvent) {
	switch e := ev.(type) {
	case transport.TextMessageEvent:
		m.messageStore.CreateIncomingMessage(
			e.Content, e.From, e.SenderName, e.To, e.Channel, e.Timestamp,
		)
	case transport.NodeUpdateEvent:
		m.nodes = m.client.State.Nodes()
		sortNodes(m.nodes)
	case transport.ConnectionEvent:
		m.connStatus = e.Status.String()
		if e.Message != "" {
			m.messageStore.CreateSystemMessage(e.Message)
		}
	case transport.TelemetryEvent:
		m.nodes = m.client.State.Nodes()
		sortNodes(m.nodes)
	case transport.PositionEvent:
		m.nodes = m.client.State.Nodes()
		sortNodes(m.nodes)
	}
}

// handleKey processes a key press and returns an optional command.
func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	// Global keys — work in any focus state.
	switch {
	case key.Matches(msg, keys.Quit):
		return tea.Quit
	case key.Matches(msg, keys.Tab):
		m.toggleFocus()
		return nil
	case key.Matches(msg, keys.NextChan):
		if len(m.channels) > 0 {
			m.channelIdx = (m.channelIdx + 1) % len(m.channels)
			m.mode = viewChannel
			m.refreshMessages()
		}
		return nil
	case key.Matches(msg, keys.PrevChan):
		if len(m.channels) > 0 {
			m.channelIdx = (m.channelIdx - 1 + len(m.channels)) % len(m.channels)
			m.mode = viewChannel
			m.refreshMessages()
		}
		return nil
	}

	// Focus-specific keys.
	switch m.focus {
	case focusSidebar:
		return m.handleSidebarKey(msg)
	case focusInput:
		return m.handleInputKey(msg)
	}
	return nil
}

// handleSidebarKey handles keys when the sidebar has focus.
func (m *Model) handleSidebarKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyUp:
		if m.sidebarCursor > 0 {
			m.sidebarCursor--
			m.ensureSidebarCursorVisible()
		}
	case tea.KeyDown:
		max := m.sidebarListLen() - 1
		if max < 0 {
			max = 0
		}
		if m.sidebarCursor < max {
			m.sidebarCursor++
			m.ensureSidebarCursorVisible()
		}
	case tea.KeyLeft, tea.KeyRight:
		// Switch between channels tab and nodes tab.
		m.sidebarTab = (m.sidebarTab + 1) % 2
		m.sidebarCursor = 0
		m.sidebarOffset = 0
	case tea.KeyEnter:
		m.selectSidebarItem()
	case tea.KeyEscape:
		m.focus = focusInput
		m.textInput.Focus()
		m.mode = viewChannel
		m.refreshMessages()
	default:
		// Any printable character → switch to input and let it be typed.
		if msg.Type == tea.KeyRunes {
			m.focus = focusInput
			m.textInput.Focus()
		}
	}
	return nil
}

// handleInputKey handles keys when the text input has focus.
func (m *Model) handleInputKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Send):
		return m.sendMessage()
	case key.Matches(msg, keys.Esc):
		if m.mode == viewDM {
			m.mode = viewChannel
			m.refreshMessages()
		}
	}
	return nil
}

// toggleFocus switches between sidebar and text input focus.
func (m *Model) toggleFocus() {
	if m.focus == focusInput {
		m.focus = focusSidebar
		m.textInput.Blur()
	} else {
		m.focus = focusInput
		m.textInput.Focus()
	}
}

// selectSidebarItem activates the currently highlighted sidebar item.
func (m *Model) selectSidebarItem() {
	if m.sidebarTab == 0 {
		// Channels tab: switch active channel.
		if m.sidebarCursor < len(m.channels) {
			m.channelIdx = m.sidebarCursor
			m.mode = viewChannel
			m.refreshMessages()
		}
	} else {
		// Nodes tab: start DM with selected node.
		if m.sidebarCursor < len(m.nodes) {
			m.dmTarget = m.nodes[m.sidebarCursor].Num
			m.mode = viewDM
			m.refreshMessages()
		}
	}
	// Return focus to input after selection.
	m.focus = focusInput
	m.textInput.Focus()
}

// sendMessage sends the current text input content via the client.
func (m *Model) sendMessage() tea.Cmd {
	content := strings.TrimSpace(m.textInput.Value())
	if content == "" {
		return nil
	}

	var to uint32
	var chIdx uint32
	switch m.mode {
	case viewChannel:
		to = store.BroadcastAddr
		chIdx = m.activeChannelIndex()
	case viewDM:
		to = m.dmTarget
		chIdx = 0 // DMs go on primary channel
	}

	m.messageStore.CreateOutgoingMessage(
		content, to, chIdx, m.client.MyNodeNum(), m.client.LocalNodeName(),
	)

	// Actually transmit over the radio.
	if err := m.client.SendText(to, chIdx, content); err != nil {
		m.messageStore.CreateSystemMessage(fmt.Sprintf("Send failed: %v", err))
	}

	m.textInput.SetValue("")
	m.refreshMessages()
	return nil
}

// ---------- View ----------

func (m Model) View() string {
	if !m.ready {
		return "Connecting to Meshtastic device…"
	}

	// Status bar
	targetLabel := m.targetLabel()
	nodeCount := len(m.nodes)
	statusText := fmt.Sprintf(" %s │ %s │ %s │ Nodes: %d ",
		m.connStatus, m.client.LocalNodeName(), targetLabel, nodeCount)
	statusBar := statusStyle.Width(m.width).Render(statusText)

	// Sidebar
	sidebar := m.renderSidebar()

	// Message viewport
	mainPanel := m.viewport.View()

	// Input
	inputBar := inputStyle.Width(m.width - sidebarW).Render(m.textInput.View())

	// Compose
	rightCol := lipgloss.JoinVertical(lipgloss.Left, mainPanel, inputBar)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, rightCol)

	// Persistent help bar
	helpBar := m.renderHelpBar()

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, body, helpBar)
}

// ---------- Sidebar rendering ----------

func (m Model) renderSidebar() string {
	// Exact body height: everything between status bar and help bar.
	bodyH := m.height - statusBarH - helpBarH
	if bodyH < 3 {
		bodyH = 3
	}

	// Content width = sidebar total width minus frame (borders + padding).
	// Each line MUST fit within this to prevent wrapping.
	contentW := sidebarW - sidebarStyle.GetHorizontalFrameSize()
	if contentW < 10 {
		contentW = 10
	}

	isSidebarFocused := m.focus == focusSidebar

	// Build list of items.
	type sidebarEntry struct {
		text string
		high bool
	}

	var header string
	var items []sidebarEntry

	if m.sidebarTab == 0 {
		if isSidebarFocused {
			header = "Channels [←→ Nodes]"
		} else {
			header = "Channels"
		}
		for i, ch := range m.channels {
			name := channelDisplayName(ch)
			active := i == m.channelIdx && m.mode == viewChannel
			cursored := isSidebarFocused && i == m.sidebarCursor
			prefix := "  "
			if cursored {
				prefix = "► "
			} else if active {
				prefix = "● "
			}
			items = append(items, sidebarEntry{prefix + name, cursored || active})
		}
	} else {
		if isSidebarFocused {
			header = "Nodes [←→ Channels]"
		} else {
			header = "Nodes"
		}
		for i, n := range m.nodes {
			name := nodeDisplayName(n)
			cursored := isSidebarFocused && i == m.sidebarCursor
			dmActive := m.mode == viewDM && n.Num == m.dmTarget
			prefix := "  "
			if cursored {
				prefix = "► "
			} else if dmActive {
				prefix = "● "
			}
			items = append(items, sidebarEntry{prefix + name, cursored || dmActive})
		}
	}

	// Build output as an exact slice of `bodyH` lines.
	// Every line is truncated to contentW to prevent wrapping.
	lines := make([]string, 0, bodyH)

	// Line 0: title (truncated).
	lines = append(lines, sidebarTitle.MaxWidth(contentW).Render(header))

	// Remaining lines for items + scroll indicators.
	itemSlots := bodyH - len(lines)
	if itemSlots < 1 {
		itemSlots = 1
	}

	total := len(items)
	offset := m.sidebarOffset

	// Determine scroll indicators and item capacity.
	hasAbove := offset > 0
	hasBelow := offset+itemSlots < total
	listCap := itemSlots
	if hasAbove {
		listCap--
	}
	if hasBelow {
		listCap--
	}
	if listCap < 1 {
		listCap = 1
	}

	// ▲ indicator.
	if hasAbove {
		lines = append(lines, sidebarScrollHint.MaxWidth(contentW).Render(fmt.Sprintf("  ▲ %d more", offset)))
	}

	// Visible items — each truncated to contentW.
	end := offset + listCap
	if end > total {
		end = total
	}
	for i := offset; i < end; i++ {
		e := items[i]
		if e.high {
			lines = append(lines, sidebarItemActive.MaxWidth(contentW).Render(e.text))
		} else {
			lines = append(lines, sidebarItem.MaxWidth(contentW).Render(e.text))
		}
	}

	// ▼ indicator.
	if hasBelow {
		remaining := total - end
		lines = append(lines, sidebarScrollHint.MaxWidth(contentW).Render(fmt.Sprintf("  ▼ %d more", remaining)))
	}

	// Pad to exactly bodyH lines so sidebar never overflows or underflows.
	for len(lines) < bodyH {
		lines = append(lines, "")
	}
	if len(lines) > bodyH {
		lines = lines[:bodyH]
	}

	content := strings.Join(lines, "\n")
	return sidebarStyle.Width(sidebarW).Render(content)
}

func (m Model) sidebarListLen() int {
	if m.sidebarTab == 0 {
		return len(m.channels)
	}
	return len(m.nodes)
}

// sidebarItemCapacity returns how many list items fit in the sidebar,
// accounting for the title line. This is the raw capacity before
// subtracting scroll indicator lines.
func (m Model) sidebarItemCapacity() int {
	bodyH := m.height - statusBarH - helpBarH
	const titleLines = 1 // header only, no padding
	v := bodyH - titleLines
	if v < 1 {
		v = 1
	}
	return v
}

// ensureSidebarCursorVisible adjusts sidebarOffset so the cursor is in view.
func (m *Model) ensureSidebarCursorVisible() {
	itemSlots := m.sidebarItemCapacity()
	// Account for scroll indicators taking a line each.
	if m.sidebarOffset > 0 {
		itemSlots--
	}
	total := m.sidebarListLen()
	if m.sidebarOffset+itemSlots < total {
		itemSlots--
	}
	if itemSlots < 1 {
		itemSlots = 1
	}

	// Scroll down if cursor is below the visible window.
	if m.sidebarCursor >= m.sidebarOffset+itemSlots {
		m.sidebarOffset = m.sidebarCursor - itemSlots + 1
	}
	// Scroll up if cursor is above the visible window.
	if m.sidebarCursor < m.sidebarOffset {
		m.sidebarOffset = m.sidebarCursor
	}
	if m.sidebarOffset < 0 {
		m.sidebarOffset = 0
	}
}

// ---------- Help bar ----------

// helpEntry is a key/description pair for the help bar.
type helpEntry struct {
	key  string
	desc string
}

// renderHelpBar returns a styled, context-sensitive help bar.
func (m Model) renderHelpBar() string {
	var entries []helpEntry

	switch m.focus {
	case focusInput:
		entries = append(entries, helpEntry{"Tab", "sidebar"})
		entries = append(entries, helpEntry{"Enter", "send"})
		entries = append(entries, helpEntry{"Ctrl+N/P", "channels"})
		if m.mode == viewDM {
			entries = append(entries, helpEntry{"Esc", "back to channel"})
		}
	case focusSidebar:
		entries = append(entries, helpEntry{"↑↓", "navigate"})
		if m.sidebarTab == 0 {
			entries = append(entries, helpEntry{"←→", "nodes"})
			entries = append(entries, helpEntry{"Enter", "switch channel"})
		} else {
			entries = append(entries, helpEntry{"←→", "channels"})
			entries = append(entries, helpEntry{"Enter", "direct message"})
		}
		entries = append(entries, helpEntry{"Tab", "input"})
		entries = append(entries, helpEntry{"Esc", "back"})
	}
	entries = append(entries, helpEntry{"Ctrl+C", "quit"})

	sep := helpSepStyle.Render(" │ ")
	var parts []string
	for _, e := range entries {
		parts = append(parts, helpKeyStyle.Render(e.key)+" "+helpDescStyle.Render(e.desc))
	}

	line := " " + strings.Join(parts, sep) + " "
	return helpBarStyle.Width(m.width).Render(line)
}

// ---------- Message rendering ----------

// refreshMessages re-renders the viewport content based on the active view.
func (m *Model) refreshMessages() {
	var msgs []store.Message
	switch m.mode {
	case viewChannel:
		msgs = m.messageStore.GetMessagesByChannel(m.activeChannelIndex())
	case viewDM:
		msgs = m.messageStore.GetMessagesByConversation(m.client.MyNodeNum(), m.dmTarget)
	}

	var lines []string
	for _, msg := range msgs {
		lines = append(lines, formatMessage(msg))
	}

	m.viewport.SetContent(strings.Join(lines, "\n"))
	m.viewport.GotoBottom()
}

func formatMessage(msg store.Message) string {
	ts := timeStyle.Render(msg.Timestamp.Format("15:04:05"))
	sender := nameStyle.Render(msg.SenderName)

	var body string
	switch msg.Type {
	case store.MessageTypeIncoming:
		body = incomingStyle.Render(msg.Content)
	case store.MessageTypeOutgoing:
		body = outgoingStyle.Render(msg.Content)
	case store.MessageTypeSystem:
		return systemStyle.Render(fmt.Sprintf("%s  %s", ts, msg.Content))
	}
	return fmt.Sprintf("%s %s %s", ts, sender, body)
}

// ---------- Helpers ----------

func (m Model) activeChannelIndex() uint32 {
	if m.channelIdx < 0 || m.channelIdx >= len(m.channels) {
		return 0
	}
	return uint32(m.channels[m.channelIdx].Index)
}

func (m Model) targetLabel() string {
	switch m.mode {
	case viewDM:
		return "DM: " + m.client.NodeName(m.dmTarget)
	default:
		if len(m.channels) > 0 && m.channelIdx < len(m.channels) {
			return "Ch: " + channelDisplayName(m.channels[m.channelIdx])
		}
		return "Ch: Primary"
	}
}

func channelDisplayName(ch *generated.Channel) string {
	if ch.Settings != nil && ch.Settings.Name != "" {
		return ch.Settings.Name
	}
	if ch.Role == generated.Channel_PRIMARY {
		return "Primary"
	}
	return fmt.Sprintf("Ch%d", ch.Index)
}

func nodeDisplayName(n *generated.NodeInfo) string {
	name := store.FormatNodeID(n.Num)
	if n.User != nil && n.User.LongName != "" {
		name = n.User.LongName
	}
	if n.HopsAway > 0 {
		name += fmt.Sprintf(" (%d hop)", n.HopsAway)
	}
	return name
}

// filterActiveChannels removes DISABLED channels from the list.
func filterActiveChannels(all []*generated.Channel) []*generated.Channel {
	var out []*generated.Channel
	for _, ch := range all {
		if ch.Role != generated.Channel_DISABLED {
			out = append(out, ch)
		}
	}
	return out
}

// sortNodes sorts nodes by last heard (most recent first), then by name A-Z.
func sortNodes(nodes []*generated.NodeInfo) {
	sort.Slice(nodes, func(i, j int) bool {
		// Most recently heard first.
		if nodes[i].LastHeard != nodes[j].LastHeard {
			return nodes[i].LastHeard > nodes[j].LastHeard
		}
		// Tie-break: alphabetical by display name.
		return nodeDisplayName(nodes[i]) < nodeDisplayName(nodes[j])
	})
}
