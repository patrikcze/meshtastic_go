// Package ui provides the terminal user interface for the Meshtastic messaging application.
package ui

import (
	"fmt"
	"sort"
	"strings"

	tui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"meshtastic_go/internal/transport"
	"meshtastic_go/internal/ui/store"
	"meshtastic_go/pkg/generated"
)

// Layout constants.
const (
	sidebarW    = 32
	inputH      = 3
	statusH     = 1
	helpH       = 1
	maxInputLen = 228 // Meshtastic max payload ~237 bytes; leave headroom.
)

// viewMode distinguishes channel view from DM view.
type viewMode int

const (
	viewChannel viewMode = iota
	viewDM
)

// focusArea tracks which panel owns keyboard input.
type focusArea int

const (
	focusInput   focusArea = iota // text input: typing, Enter sends
	focusSidebar                  // sidebar: arrows navigate, Enter selects
)

// App is the main TUI application.
type App struct {
	client *transport.Client

	width, height int
	sideW         int // effective sidebar width after layout

	// Messaging state
	messageStore *store.MessageStore
	mode         viewMode
	channelIdx   int    // index into the channels slice for the active channel
	dmTarget     uint32 // node number of DM peer (when mode == viewDM)

	// Focus & sidebar state
	focus         focusArea
	sidebarTab    int // 0 = channels, 1 = nodes
	sidebarCursor int
	sidebarOffset int // first visible item index

	// Cached data from client.State (refreshed on events)
	channels []*generated.Channel
	nodes    []*generated.NodeInfo

	// Widgets
	statusBar *widgets.Paragraph
	sidebar   *widgets.Paragraph
	msgView   *widgets.Paragraph
	inputBox  *widgets.Paragraph
	helpBar   *widgets.Paragraph

	// Text input state
	inputBuf    []rune
	inputCursor int

	// Message display state
	msgLines  []string
	msgScroll int // offset from bottom (0 = latest messages visible)

	connStatus string
}

// New creates an App wired to the given connected Client.
func New(client *transport.Client) *App {
	ms := store.NewMessageStore()
	ms.CreateSystemMessage("Welcome to Meshtastic Go!")

	channels := filterActiveChannels(client.State.Channels())
	nodes := client.State.Nodes()
	sortNodes(nodes)

	return &App{
		client:       client,
		messageStore: ms,
		mode:         viewChannel,
		channels:     channels,
		nodes:        nodes,
		statusBar:    widgets.NewParagraph(),
		sidebar:      widgets.NewParagraph(),
		msgView:      widgets.NewParagraph(),
		inputBox:     widgets.NewParagraph(),
		helpBar:      widgets.NewParagraph(),
		connStatus:   "Connected",
	}
}

// Run initialises termui and drives the event loop until quit.
func (a *App) Run() error {
	if err := tui.Init(); err != nil {
		return fmt.Errorf("initialising terminal UI: %w", err)
	}
	defer tui.Close()

	a.initWidgets()
	a.width, a.height = tui.TerminalDimensions()
	a.layout()
	a.refreshMessages()
	a.draw()

	uiEvents := tui.PollEvents()
	radioEvents := a.client.Events // will be nil-ed after close

	for {
		select {
		case e := <-uiEvents:
			switch e.Type {
			case tui.KeyboardEvent:
				if a.handleKey(e.ID) {
					return nil
				}
			case tui.ResizeEvent:
				p := e.Payload.(tui.Resize)
				a.width, a.height = p.Width, p.Height
				a.layout()
				a.refreshMessages()
			case tui.MouseEvent:
				a.handleMouse(e.ID)
			}
			a.draw()

		case ev, ok := <-radioEvents:
			if !ok {
				radioEvents = nil // prevent busy-loop on closed channel
				a.connStatus = "Disconnected"
				a.messageStore.CreateSystemMessage("Radio connection lost")
				a.refreshMessages()
				a.draw()
				continue
			}
			a.handleRadioEvent(ev)
			a.refreshMessages()
			a.draw()
		}
	}
}

// ---------- Initialisation ----------

func (a *App) initWidgets() {
	// Status bar — borderless, coloured background.
	a.statusBar.Border = false
	a.statusBar.TextStyle = tui.NewStyle(tui.ColorWhite, tui.Color(24))

	// Sidebar
	a.sidebar.Title = " Channels "
	a.sidebar.TitleStyle = tui.NewStyle(tui.Color(39), tui.ColorClear, tui.ModifierBold)
	a.sidebar.BorderStyle = tui.NewStyle(tui.Color(240))
	a.sidebar.WrapText = false

	// Message viewport
	a.msgView.Title = " Messages "
	a.msgView.TitleStyle = tui.NewStyle(tui.Color(39), tui.ColorClear, tui.ModifierBold)
	a.msgView.BorderStyle = tui.NewStyle(tui.Color(240))
	a.msgView.WrapText = false

	// Text input
	a.inputBox.BorderStyle = tui.NewStyle(tui.Color(39))
	a.inputBox.WrapText = false

	// Help bar — borderless, coloured background.
	a.helpBar.Border = false
	a.helpBar.TextStyle = tui.NewStyle(tui.Color(252), tui.Color(238))
}

// layout repositions all widgets for the current terminal size.
func (a *App) layout() {
	w, h := a.width, a.height
	if w < 40 {
		w = 40
	}
	if h < 8 {
		h = 8
	}

	sw := sidebarW
	if sw > w/2 {
		sw = w / 2
	}
	a.sideW = sw

	a.statusBar.SetRect(0, 0, w, statusH)
	a.sidebar.SetRect(0, statusH, sw, h-helpH)
	a.msgView.SetRect(sw, statusH, w, h-helpH-inputH)
	a.inputBox.SetRect(sw, h-helpH-inputH, w, h-helpH)
	a.helpBar.SetRect(0, h-helpH, w, h)
}

// ---------- Rendering ----------

func (a *App) draw() {
	tui.Clear()
	a.renderStatusBar()
	a.renderSidebar()
	a.renderMsgView()
	a.renderInput()
	a.renderHelpBar()
	tui.Render(a.statusBar, a.sidebar, a.msgView, a.inputBox, a.helpBar)
}

func (a *App) renderStatusBar() {
	target := a.targetLabel()
	nodeCount := len(a.nodes)
	a.statusBar.Text = fmt.Sprintf(" %s │ %s │ %s │ Nodes: %d",
		a.connStatus, a.client.LocalNodeName(), target, nodeCount)
}

func (a *App) renderSidebar() {
	isFocused := a.focus == focusSidebar

	// Title reflects current tab and available switch.
	if a.sidebarTab == 0 {
		if isFocused {
			a.sidebar.Title = " Channels [←→ Nodes] "
		} else {
			a.sidebar.Title = " Channels "
		}
	} else {
		if isFocused {
			a.sidebar.Title = " Nodes [←→ Channels] "
		} else {
			a.sidebar.Title = " Nodes "
		}
	}

	// Highlight border when focused.
	if isFocused {
		a.sidebar.BorderStyle = tui.NewStyle(tui.Color(39))
	} else {
		a.sidebar.BorderStyle = tui.NewStyle(tui.Color(240))
	}

	// Build item list.
	type entry struct {
		text string
		high bool
	}
	var items []entry

	if a.sidebarTab == 0 {
		for i, ch := range a.channels {
			name := channelDisplayName(ch)
			active := i == a.channelIdx && a.mode == viewChannel
			cursored := isFocused && i == a.sidebarCursor
			prefix := "  "
			if cursored {
				prefix = "► "
			} else if active {
				prefix = "● "
			}
			items = append(items, entry{prefix + name, cursored || active})
		}
	} else {
		for i, n := range a.nodes {
			name := nodeDisplayName(n)
			cursored := isFocused && i == a.sidebarCursor
			dmActive := a.mode == viewDM && n.Num == a.dmTarget
			prefix := "  "
			if cursored {
				prefix = "► "
			} else if dmActive {
				prefix = "● "
			}
			items = append(items, entry{prefix + name, cursored || dmActive})
		}
	}

	// Scrollable window.
	contentH := a.sidebarContentH()
	total := len(items)
	offset := a.sidebarOffset

	hasAbove := offset > 0
	hasBelow := offset+contentH < total
	listCap := contentH
	if hasAbove {
		listCap--
	}
	if hasBelow {
		listCap--
	}
	if listCap < 1 {
		listCap = 1
	}

	contentW := a.sideW - 2 // minus borders
	if contentW < 10 {
		contentW = 10
	}

	var lines []string

	if hasAbove {
		lines = append(lines, fmt.Sprintf("[  ▲ %d more](fg:243)", offset))
	}

	end := offset + listCap
	if end > total {
		end = total
	}
	for i := offset; i < end; i++ {
		e := items[i]
		t := truncateStr(e.text, contentW)
		if e.high {
			lines = append(lines, fmt.Sprintf("[%s](fg:229,mod:bold)", t))
		} else {
			lines = append(lines, fmt.Sprintf("[%s](fg:white)", t))
		}
	}

	if hasBelow {
		remaining := total - end
		lines = append(lines, fmt.Sprintf("[  ▼ %d more](fg:243)", remaining))
	}

	a.sidebar.Text = strings.Join(lines, "\n")
}

func (a *App) renderMsgView() {
	// Dynamic title mirrors the active target.
	switch a.mode {
	case viewDM:
		a.msgView.Title = fmt.Sprintf(" DM: %s ", a.client.NodeName(a.dmTarget))
	default:
		if len(a.channels) > 0 && a.channelIdx < len(a.channels) {
			a.msgView.Title = fmt.Sprintf(" %s ", channelDisplayName(a.channels[a.channelIdx]))
		} else {
			a.msgView.Title = " Messages "
		}
	}

	contentH := a.msgContentH()
	total := len(a.msgLines)
	if total == 0 {
		a.msgView.Text = ""
		return
	}

	endIdx := total - a.msgScroll
	if endIdx > total {
		endIdx = total
	}
	if endIdx < 0 {
		endIdx = 0
	}
	startIdx := endIdx - contentH
	if startIdx < 0 {
		startIdx = 0
	}

	a.msgView.Text = strings.Join(a.msgLines[startIdx:endIdx], "\n")
}

func (a *App) renderInput() {
	if a.focus == focusInput {
		a.inputBox.BorderStyle = tui.NewStyle(tui.Color(39))
		if len(a.inputBuf) == 0 {
			a.inputBox.Text = "[█](fg:39)"
		} else {
			before := string(a.inputBuf[:a.inputCursor])
			after := string(a.inputBuf[a.inputCursor:])
			a.inputBox.Text = safeMarkup(before) + "[█](fg:39)" + safeMarkup(after)
		}
	} else {
		a.inputBox.BorderStyle = tui.NewStyle(tui.Color(240))
		if len(a.inputBuf) == 0 {
			a.inputBox.Text = "[Type a message…](fg:243)"
		} else {
			a.inputBox.Text = safeMarkup(string(a.inputBuf))
		}
	}
}

func (a *App) renderHelpBar() {
	var parts []string
	switch a.focus {
	case focusInput:
		parts = append(parts, "[Tab](fg:229,mod:bold) sidebar")
		parts = append(parts, "[Enter](fg:229,mod:bold) send")
		parts = append(parts, "[Ctrl+N/P](fg:229,mod:bold) channels")
		if a.mode == viewDM {
			parts = append(parts, "[Esc](fg:229,mod:bold) back to channel")
		}
	case focusSidebar:
		parts = append(parts, "[↑↓](fg:229,mod:bold) navigate")
		if a.sidebarTab == 0 {
			parts = append(parts, "[←→](fg:229,mod:bold) nodes")
			parts = append(parts, "[Enter](fg:229,mod:bold) switch channel")
		} else {
			parts = append(parts, "[←→](fg:229,mod:bold) channels")
			parts = append(parts, "[Enter](fg:229,mod:bold) direct message")
		}
		parts = append(parts, "[Tab](fg:229,mod:bold) input")
		parts = append(parts, "[Esc](fg:229,mod:bold) back")
	}
	parts = append(parts, "[Ctrl+C](fg:229,mod:bold) quit")

	a.helpBar.Text = " " + strings.Join(parts, " [│](fg:243) ")
}

// ---------- Event handling ----------

// handleKey processes a keyboard event. Returns true to quit.
func (a *App) handleKey(id string) bool {
	// Global keys.
	switch id {
	case "<C-c>":
		return true
	case "<Tab>":
		a.toggleFocus()
		return false
	case "<C-n>":
		if len(a.channels) > 0 {
			a.channelIdx = (a.channelIdx + 1) % len(a.channels)
			a.mode = viewChannel
			a.refreshMessages()
		}
		return false
	case "<C-p>":
		if len(a.channels) > 0 {
			a.channelIdx = (a.channelIdx - 1 + len(a.channels)) % len(a.channels)
			a.mode = viewChannel
			a.refreshMessages()
		}
		return false
	}

	// Focus-specific keys.
	switch a.focus {
	case focusSidebar:
		a.handleSidebarKey(id)
	case focusInput:
		a.handleInputKey(id)
	}
	return false
}

func (a *App) handleSidebarKey(id string) {
	switch id {
	case "<Up>":
		if a.sidebarCursor > 0 {
			a.sidebarCursor--
			a.ensureSidebarVisible()
		}
	case "<Down>":
		mx := a.sidebarListLen() - 1
		if mx < 0 {
			mx = 0
		}
		if a.sidebarCursor < mx {
			a.sidebarCursor++
			a.ensureSidebarVisible()
		}
	case "<Left>", "<Right>":
		a.sidebarTab = (a.sidebarTab + 1) % 2
		a.sidebarCursor = 0
		a.sidebarOffset = 0
	case "<Enter>":
		a.selectSidebarItem()
	case "<Escape>":
		a.focus = focusInput
		a.mode = viewChannel
		a.refreshMessages()
	default:
		// Printable character → switch to input and type it.
		if len(id) > 0 && id[0] != '<' {
			a.focus = focusInput
			a.handleInputKey(id)
		}
	}
}

func (a *App) handleInputKey(id string) {
	switch id {
	case "<Enter>":
		a.sendMessage()
	case "<Escape>":
		if a.mode == viewDM {
			a.mode = viewChannel
			a.refreshMessages()
		}
	case "<Backspace>", "<C-h>":
		if a.inputCursor > 0 {
			a.inputBuf = append(a.inputBuf[:a.inputCursor-1], a.inputBuf[a.inputCursor:]...)
			a.inputCursor--
		}
	case "<Delete>":
		if a.inputCursor < len(a.inputBuf) {
			a.inputBuf = append(a.inputBuf[:a.inputCursor], a.inputBuf[a.inputCursor+1:]...)
		}
	case "<Left>":
		if a.inputCursor > 0 {
			a.inputCursor--
		}
	case "<Right>":
		if a.inputCursor < len(a.inputBuf) {
			a.inputCursor++
		}
	case "<Home>", "<C-a>":
		a.inputCursor = 0
	case "<End>", "<C-e>":
		a.inputCursor = len(a.inputBuf)
	case "<C-u>":
		a.inputBuf = a.inputBuf[:0]
		a.inputCursor = 0
	case "<C-k>":
		a.inputBuf = a.inputBuf[:a.inputCursor]
	case "<C-w>":
		if a.inputCursor > 0 {
			i := a.inputCursor - 1
			for i > 0 && a.inputBuf[i] == ' ' {
				i--
			}
			for i > 0 && a.inputBuf[i-1] != ' ' {
				i--
			}
			a.inputBuf = append(a.inputBuf[:i], a.inputBuf[a.inputCursor:]...)
			a.inputCursor = i
		}
	case "<Space>":
		a.insertRune(' ')
	case "<PageUp>":
		a.scrollMessages(5)
	case "<PageDown>":
		a.scrollMessages(-5)
	default:
		// Regular character input (skip anything that looks like a special key).
		if len(id) > 0 && id[0] != '<' {
			for _, r := range id {
				a.insertRune(r)
			}
		}
	}
}

func (a *App) handleMouse(id string) {
	switch id {
	case "<MouseWheelUp>":
		a.scrollMessages(3)
		a.draw()
	case "<MouseWheelDown>":
		a.scrollMessages(-3)
		a.draw()
	}
}

func (a *App) handleRadioEvent(ev transport.RadioEvent) {
	switch e := ev.(type) {
	case transport.TextMessageEvent:
		a.messageStore.CreateIncomingMessage(
			e.Content, e.From, e.SenderName, e.To, e.Channel, e.Timestamp,
		)
	case transport.NodeUpdateEvent:
		a.nodes = a.client.State.Nodes()
		sortNodes(a.nodes)
	case transport.ConnectionEvent:
		a.connStatus = e.Status.String()
		if e.Message != "" {
			a.messageStore.CreateSystemMessage(e.Message)
		}
	case transport.TelemetryEvent:
		a.nodes = a.client.State.Nodes()
		sortNodes(a.nodes)
	case transport.PositionEvent:
		a.nodes = a.client.State.Nodes()
		sortNodes(a.nodes)
	}
}

// ---------- Actions ----------

func (a *App) toggleFocus() {
	if a.focus == focusInput {
		a.focus = focusSidebar
	} else {
		a.focus = focusInput
	}
}

func (a *App) selectSidebarItem() {
	if a.sidebarTab == 0 {
		if a.sidebarCursor < len(a.channels) {
			a.channelIdx = a.sidebarCursor
			a.mode = viewChannel
			a.refreshMessages()
		}
	} else {
		if a.sidebarCursor < len(a.nodes) {
			a.dmTarget = a.nodes[a.sidebarCursor].Num
			a.mode = viewDM
			a.refreshMessages()
		}
	}
	a.focus = focusInput
}

func (a *App) sendMessage() {
	content := strings.TrimSpace(string(a.inputBuf))
	if content == "" {
		return
	}

	var to, chIdx uint32
	switch a.mode {
	case viewChannel:
		to = store.BroadcastAddr
		chIdx = a.activeChannelIndex()
	case viewDM:
		to = a.dmTarget
		chIdx = 0
	}

	a.messageStore.CreateOutgoingMessage(
		content, to, chIdx, a.client.MyNodeNum(), a.client.LocalNodeName(),
	)
	if err := a.client.SendText(to, chIdx, content); err != nil {
		a.messageStore.CreateSystemMessage(fmt.Sprintf("Send failed: %v", err))
	}

	a.inputBuf = a.inputBuf[:0]
	a.inputCursor = 0
	a.refreshMessages()
}

func (a *App) insertRune(r rune) {
	if len(a.inputBuf) >= maxInputLen {
		return
	}
	newBuf := make([]rune, len(a.inputBuf)+1)
	copy(newBuf, a.inputBuf[:a.inputCursor])
	newBuf[a.inputCursor] = r
	copy(newBuf[a.inputCursor+1:], a.inputBuf[a.inputCursor:])
	a.inputBuf = newBuf
	a.inputCursor++
}

// ---------- Messages ----------

func (a *App) refreshMessages() {
	var msgs []store.Message
	switch a.mode {
	case viewChannel:
		msgs = a.messageStore.GetMessagesByChannel(a.activeChannelIndex())
	case viewDM:
		msgs = a.messageStore.GetMessagesByConversation(a.client.MyNodeNum(), a.dmTarget)
	}

	lines := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		lines = append(lines, formatMessage(msg))
	}
	a.msgLines = lines
	a.msgScroll = 0 // pin to latest
}

func (a *App) scrollMessages(delta int) {
	a.msgScroll += delta
	maxScroll := len(a.msgLines) - a.msgContentH()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.msgScroll > maxScroll {
		a.msgScroll = maxScroll
	}
	if a.msgScroll < 0 {
		a.msgScroll = 0
	}
}

func formatMessage(msg store.Message) string {
	ts := msg.Timestamp.Format("15:04:05")
	sender := padRight(msg.SenderName, 16)

	switch msg.Type {
	case store.MessageTypeSystem:
		return fmt.Sprintf("[%s  %s](fg:yellow)", ts, safeMarkup(msg.Content))
	case store.MessageTypeIncoming:
		return fmt.Sprintf("[%s](fg:white) [%s](fg:cyan,mod:bold) [%s](fg:green)",
			ts, safeMarkup(sender), safeMarkup(msg.Content))
	case store.MessageTypeOutgoing:
		return fmt.Sprintf("[%s](fg:white) [%s](fg:cyan,mod:bold) [%s](fg:magenta)",
			ts, safeMarkup(sender), safeMarkup(msg.Content))
	default:
		return fmt.Sprintf("[%s](fg:white) [%s](fg:cyan,mod:bold) [%s](fg:white)",
			ts, safeMarkup(sender), safeMarkup(msg.Content))
	}
}

// ---------- Sidebar scrolling ----------

func (a *App) sidebarListLen() int {
	if a.sidebarTab == 0 {
		return len(a.channels)
	}
	return len(a.nodes)
}

func (a *App) sidebarContentH() int {
	h := a.height - statusH - helpH - 2 // border top/bottom
	if h < 1 {
		h = 1
	}
	return h
}

func (a *App) msgContentH() int {
	h := a.height - statusH - helpH - inputH - 2 // border top/bottom of msgView
	if h < 1 {
		h = 1
	}
	return h
}

func (a *App) ensureSidebarVisible() {
	cap := a.sidebarContentH()
	if a.sidebarOffset > 0 {
		cap--
	}
	total := a.sidebarListLen()
	if a.sidebarOffset+cap < total {
		cap--
	}
	if cap < 1 {
		cap = 1
	}
	if a.sidebarCursor >= a.sidebarOffset+cap {
		a.sidebarOffset = a.sidebarCursor - cap + 1
	}
	if a.sidebarCursor < a.sidebarOffset {
		a.sidebarOffset = a.sidebarCursor
	}
	if a.sidebarOffset < 0 {
		a.sidebarOffset = 0
	}
}

// ---------- Helpers ----------

func (a *App) activeChannelIndex() uint32 {
	if a.channelIdx < 0 || a.channelIdx >= len(a.channels) {
		return 0
	}
	return uint32(a.channels[a.channelIdx].Index)
}

func (a *App) targetLabel() string {
	switch a.mode {
	case viewDM:
		return "DM: " + a.client.NodeName(a.dmTarget)
	default:
		if len(a.channels) > 0 && a.channelIdx < len(a.channels) {
			return "Ch: " + channelDisplayName(a.channels[a.channelIdx])
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

func filterActiveChannels(all []*generated.Channel) []*generated.Channel {
	var out []*generated.Channel
	for _, ch := range all {
		if ch.Role != generated.Channel_DISABLED {
			out = append(out, ch)
		}
	}
	return out
}

func sortNodes(nodes []*generated.NodeInfo) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].LastHeard != nodes[j].LastHeard {
			return nodes[i].LastHeard > nodes[j].LastHeard
		}
		return nodeDisplayName(nodes[i]) < nodeDisplayName(nodes[j])
	})
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		if maxLen > 1 {
			return string(runes[:maxLen-1]) + "…"
		}
		return string(runes[:maxLen])
	}
	return s
}

func padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	return s + strings.Repeat(" ", width-len(runes))
}

// safeMarkup prevents user content from being parsed as termui style markup
// by breaking any accidental ](fg: patterns.
func safeMarkup(s string) string {
	return strings.ReplaceAll(s, "](", "] (")
}
