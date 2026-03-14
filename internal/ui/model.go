// Package ui provides the terminal user interface for the Meshtastic messaging application.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"google.golang.org/protobuf/proto"

	"meshtastic_go/internal/transport"
	"meshtastic_go/internal/ui/store"
	"meshtastic_go/pkg/generated"
	"meshtastic_go/pkg/serial"
)

// Constants for styling
const (
	// Default width, height will be determined by terminal size
	defaultWidth  = 120
	defaultHeight = 40
)

// UI area sizes
const (
	statusBarHeight = 1
	inputAreaHeight = 3
	sidebarWidth    = 30
)

// Custom messages for the tea.Model
type (
	// ErrorMsg represents an error that occurred
	ErrorMsg struct{ err error }
	// ConnectionStatusMsg updates the connection status
	ConnectionStatusMsg string
	// NewMessageMsg indicates a new message was received
	NewMessageMsg store.Message
	// NodeListUpdatedMsg indicates the node list was updated
	NodeListUpdatedMsg []string
)

// Key bindings
type keyMap struct {
	Help     key.Binding
	Quit     key.Binding
	SendMsg  key.Binding
	NextChan key.Binding
	PrevChan key.Binding
}

// Default key mappings
var keys = keyMap{
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	SendMsg: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "send message"),
	),
	NextChan: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next channel"),
	),
	PrevChan: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev channel"),
	),
}

// ShortHelp returns keybindings to be shown in the mini help view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.SendMsg}
}

// FullHelp returns keybindings for the expanded help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit},
		{k.SendMsg, k.NextChan, k.PrevChan},
	}
}

// Styles for UI components
var (
	// Base style
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	// Status bar style
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("240")).
			Bold(true).
			Width(defaultWidth)

	// Input area style
	inputAreaStyle = baseStyle.Copy().
			BorderTop(true).
			BorderLeft(false).
			BorderRight(false).
			BorderBottom(false).
			Padding(0, 1)

	// Message view style
	messageViewStyle = baseStyle.Copy().
				BorderLeft(false).
				BorderRight(false).
				BorderTop(false).
				BorderBottom(false).
				Padding(1, 2)

	// Sidebar style
	sidebarStyle = baseStyle.Copy().
			BorderLeft(false).
			BorderTop(false).
			BorderBottom(false).
			BorderRight(true).
			Padding(1, 1).
			Width(sidebarWidth)

	// Message item styles
	incomingMsgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("99")).
				PaddingLeft(1)

	outgoingMsgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("170")).
				PaddingLeft(1)

	systemMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true).
			PaddingLeft(1)

	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(8)

	senderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true).
			Width(15)
)

// Model represents the main application UI model.
type Model struct {
	// Application state
	width            int
	height           int
	ready            bool
	showHelp         bool
	err              error
	connectionStatus string
	activeChannelID  int
	localNodeID      uint32
	localNodeName    string

	// UI components
	help            help.Model
	messageViewport viewport.Model
	textInput       textinput.Model
	nodeList        list.Model
	channelList     list.Model

	// Data stores
	messageStore *store.MessageStore
	nodeInfo     map[uint32]string
	channels     []*generated.Channel

	// Meshtastic client
	client     *transport.Client
	streamConn *transport.StreamConn
}

// New creates a new Model with default values.
func New() Model {
	// Initialize UI components
	help := help.New()
	help.ShowAll = false

	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "Type your message..."
	ti.Focus()
	ti.CharLimit = 250
	ti.Width = defaultWidth - sidebarWidth

	// Initialize node list
	nodeList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	nodeList.Title = "Connected Nodes"
	nodeList.SetShowHelp(false)

	// Initialize channel list
	channelList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	channelList.Title = "Channels"

	// Initialize viewport for message display
	mv := viewport.New(defaultWidth-sidebarWidth, defaultHeight-statusBarHeight-inputAreaHeight)
	mv.SetContent("")

	// Initialize message store
	ms := store.NewMessageStore()

	// Add a welcome message
	ms.CreateSystemMessage("Welcome to Meshtastic Go Messenger!")
	ms.CreateSystemMessage("Searching for Meshtastic devices...")

	return Model{
		// Set default values
		width:            defaultWidth,
		height:           defaultHeight,
		ready:            false,
		showHelp:         false,
		connectionStatus: "Disconnected",
		activeChannelID:  0, // Default channel

		// Initialize components
		help:            help,
		messageViewport: mv,
		textInput:       ti,
		nodeList:        nodeList,
		channelList:     channelList,

		// Initialize stores
		messageStore: ms,
		nodeInfo:     make(map[uint32]string),
		channels:     make([]*generated.Channel, 0),
	}
}

// Init initializes the model and returns commands to run.
func (m Model) Init() tea.Cmd {
	// Add commands to run at startup
	return tea.Batch(
		// Start device detection and connection in background
		m.detectAndConnectCmd(),
		// Set up a ticker for periodic UI updates
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tea.Tick(time.Second, func(time.Time) tea.Msg { return nil })
		}),
	)
}

// detectAndConnectCmd creates a command that attempts to detect and connect to Meshtastic devices.
func (m Model) detectAndConnectCmd() tea.Cmd {
	return func() tea.Msg {
		// Detect available ports
		ports := serial.GetPorts()
		if len(ports) == 0 {
			return ErrorMsg{fmt.Errorf("no Meshtastic devices found")}
		}

		// Use the first port
		port := ports[0]
		streamPort, err := serial.Connect(port)
		if err != nil {
			return ErrorMsg{fmt.Errorf("failed to connect to device on %s: %v", port, err)}
		}

		// Create stream connection and client
		streamConn := transport.NewRadioStreamConn(streamPort)
		client := transport.NewClient(streamConn, false)

		// Register message handler
		client.Handle(&generated.MeshPacket{}, func(msg proto.Message) {
			if packet, ok := msg.(*generated.MeshPacket); ok {
				// Process the received message and add it to the message store
				m.messageStore.AddFromMeshPacket(packet, m.nodeInfo)
			}
		})

		// Connect to the device
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := client.Connect(ctx); err != nil {
			return ErrorMsg{fmt.Errorf("failed to establish connection: %v", err)}
		}

		// Update local node info
		if info := client.State.NodeInfo(); info != nil {
			m.localNodeID = info.MyNodeNum

			// Try to get a user-friendly name from the device metadata
			if meta := client.State.DeviceMetadata(); meta != nil {
				// Use firmware version as our display name
				m.localNodeName = meta.FirmwareVersion

				// Check if we can get node info from the state
				nodeInfos := client.State.Nodes()
				for _, node := range nodeInfos {
					if node.Num == m.localNodeID && node.User != nil && node.User.LongName != "" {
						// Use the user's long name if available
						m.localNodeName = node.User.LongName
						break
					}
				}
			} else {
				// Fallback to a node ID if no metadata is available
				m.localNodeName = fmt.Sprintf("Local-%d", m.localNodeID)
			}
		}

		// Update channel info
		m.channels = client.State.Channels()

		// Store client and connection for later use
		m.client = client
		m.streamConn = streamConn

		// Update node list
		var nodeItems []list.Item
		for id, name := range m.nodeInfo {
			nodeItems = append(nodeItems, nodeListItem{id: id, name: name})
		}
		m.nodeList.SetItems(nodeItems)

		// Return connection status message
		return ConnectionStatusMsg("Connected to " + port)
	}
}

// Update handles events and updates the model accordingly.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp

		case key.Matches(msg, keys.SendMsg):
			if m.client != nil && m.textInput.Value() != "" {
				// Create and send the message
				content := m.textInput.Value()
				m.messageStore.CreateOutgoingMessage(
					content,
					0xFFFFFFFF, // Broadcast to all
					m.activeChannelID,
					m.localNodeID,
					m.localNodeName,
				)

				// TODO: Implement the actual sending through the Meshtastic client
				// This will be done in a separate function

				// Clear the input
				m.textInput.SetValue("")

				// Update the message viewport with the new message
				cmd = m.updateMessageViewCmd()
				cmds = append(cmds, cmd)
			}

		case key.Matches(msg, keys.NextChan):
			// Change to next channel if available
			if len(m.channels) > 0 {
				m.activeChannelID = (m.activeChannelID + 1) % len(m.channels)
				cmd = m.updateMessageViewCmd()
				cmds = append(cmds, cmd)
			}

		case key.Matches(msg, keys.PrevChan):
			// Change to previous channel if available
			if len(m.channels) > 0 {
				m.activeChannelID = (m.activeChannelID - 1 + len(m.channels)) % len(m.channels)
				cmd = m.updateMessageViewCmd()
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		// Update the model's window size
		m.width = msg.Width
		m.height = msg.Height

		// Adjust viewport and other component sizes
		headerHeight := statusBarHeight
		footerHeight := inputAreaHeight
		viewportHeight := m.height - headerHeight - footerHeight

		m.messageViewport.Width = m.width - sidebarWidth
		m.messageViewport.Height = viewportHeight

		// Update the status bar width
		statusBarStyle = statusBarStyle.Width(m.width)

		// Update the text input width
		m.textInput.Width = m.width - sidebarWidth - 4

		// Update the node list dimensions
		m.nodeList.SetSize(sidebarWidth, m.height-headerHeight)

		// Mark as ready to render with correct dimensions
		m.ready = true

		// Update the message viewport content
		cmd = m.updateMessageViewCmd()
		cmds = append(cmds, cmd)

	case ErrorMsg:
		// Update the error state
		m.err = msg.err
		// Add an error message to the message store
		m.messageStore.CreateSystemMessage(fmt.Sprintf("Error: %v", msg.err))
		// Update the message viewport
		cmd = m.updateMessageViewCmd()
		cmds = append(cmds, cmd)

	case ConnectionStatusMsg:
		// Update the connection status
		m.connectionStatus = string(msg)
		// Add a system message about the connection
		m.messageStore.CreateSystemMessage(fmt.Sprintf("Connection status: %s", msg))
		// Update the message viewport
		cmd = m.updateMessageViewCmd()
		cmds = append(cmds, cmd)

	case NewMessageMsg:
		// New message received, update the message viewport
		cmd = m.updateMessageViewCmd()
		cmds = append(cmds, cmd)

	case NodeListUpdatedMsg:
		// Update the node list items
		var items []list.Item
		for _, name := range msg {
			items = append(items, nodeListItem{name: name})
		}
		m.nodeList.SetItems(items)
	}

	// Update child components
	var textInputCmd tea.Cmd
	m.textInput, textInputCmd = m.textInput.Update(msg)
	cmds = append(cmds, textInputCmd)

	var viewportCmd tea.Cmd
	m.messageViewport, viewportCmd = m.messageViewport.Update(msg)
	cmds = append(cmds, viewportCmd)

	var nodeListCmd tea.Cmd
	m.nodeList, nodeListCmd = m.nodeList.Update(msg)
	cmds = append(cmds, nodeListCmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Build the status bar
	var channelInfo string
	if len(m.channels) > 0 && m.activeChannelID < len(m.channels) {
		if m.channels[m.activeChannelID].Settings != nil {
			channelInfo = m.channels[m.activeChannelID].Settings.Name
		} else {
			channelInfo = fmt.Sprintf("Ch%d", m.activeChannelID)
		}
	} else {
		channelInfo = "Default"
	}
	statusBar := statusBarStyle.Render(fmt.Sprintf(" Status: %s | Channel: %s | Nodes: %d ",
		m.connectionStatus,
		channelInfo,
		len(m.nodeInfo),
	))

	// Render the help text if it's enabled
	var helpView string
	if m.showHelp {
		helpView = m.help.View(keys)
	}

	// Determine the active panel
	messagePanelContent := m.messageViewport.View()
	messagePanel := messageViewStyle.Height(m.messageViewport.Height).Width(m.width - sidebarWidth).Render(messagePanelContent)

	// Render the input area
	inputArea := inputAreaStyle.Width(m.width - sidebarWidth).Render(m.textInput.View())

	// Render the sidebar with node list
	sidebar := sidebarStyle.Height(m.height - statusBarHeight).Render(m.nodeList.View())

	// Combine the status bar, viewport, sidebar, and input area
	mainContent := lipgloss.JoinVertical(
		lipgloss.Left,
		messagePanel,
		inputArea,
	)

	// Join sidebar and main content horizontally
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebar,
		mainContent,
	)

	// Combine everything
	return lipgloss.JoinVertical(
		lipgloss.Left,
		statusBar,
		content,
		helpView,
	)
}

// updateMessageViewCmd creates a command to update the message viewport with current messages.
func (m Model) updateMessageViewCmd() tea.Cmd {
	return func() tea.Msg {
		// Get all messages or filter by active channel, if needed
		messages := m.messageStore.GetAllMessages()

		// Format the messages for display
		var formattedMessages []string
		for _, msg := range messages {
			formattedMsg := formatMessage(msg)
			formattedMessages = append(formattedMessages, formattedMsg)
		}

		// Join the formatted messages into a single string
		content := strings.Join(formattedMessages, "\n")

		// Set the content of the viewport
		m.messageViewport.SetContent(content)

		// Scroll to the bottom to see newest messages
		m.messageViewport.GotoBottom()

		return nil
	}
}

// formatMessage formats a message for display in the viewport.
func formatMessage(msg store.Message) string {
	// Format timestamp
	timestamp := timestampStyle.Render(msg.Timestamp.Format("15:04:05"))

	// Format sender
	sender := senderStyle.Render(msg.SenderName)

	// Format message based on type
	var formattedContent string
	switch msg.Type {
	case store.MessageTypeIncoming:
		formattedContent = incomingMsgStyle.Render(msg.Content)
	case store.MessageTypeOutgoing:
		formattedContent = outgoingMsgStyle.Render(msg.Content)
	case store.MessageTypeSystem:
		formattedContent = systemMsgStyle.Render(msg.Content)
	}

	// Combine all parts
	return fmt.Sprintf("%s %s: %s", timestamp, sender, formattedContent)
}

// nodeListItem implements list.Item for the node list.
type nodeListItem struct {
	id   uint32
	name string
}

// FilterValue implements list.Item.
func (n nodeListItem) FilterValue() string {
	return n.name
}

// Title implements list.Item.
func (n nodeListItem) Title() string {
	return n.name
}

// Description implements list.Item.
func (n nodeListItem) Description() string {
	return fmt.Sprintf("ID: %d", n.id)
}

// NodeItem represents a node in the list
type NodeItem struct {
	Name string
}

// FilterValue implements list.Item
func (n NodeItem) FilterValue() string {
	return n.Name
}

// Title implements list.Item
func (n NodeItem) Title() string {
	return n.Name
}

// Description implements list.Item
func (n NodeItem) Description() string {
	return "Connected Node"
}

// ChannelItem represents a channel in the list
type ChannelItem struct {
	Name string
}

// FilterValue implements list.Item
func (c ChannelItem) FilterValue() string {
	return c.Name
}

// Title implements list.Item
func (c ChannelItem) Title() string {
	return c.Name
}

// Description implements list.Item
func (c ChannelItem) Description() string {
	return "Channel"
}

// UpdateNodes updates the node list in the TUI
func (m *Model) UpdateNodes(nodes []string) {
	items := make([]list.Item, len(nodes))
	for i, node := range nodes {
		items[i] = NodeItem{Name: node}
	}
	m.nodeList.SetItems(items)
}

// UpdateChannels updates the channel list in the TUI
func (m *Model) UpdateChannels(channels []string) {
	items := make([]list.Item, len(channels))
	for i, channel := range channels {
		items[i] = ChannelItem{Name: channel}
	}
	m.channelList.SetItems(items)
}
