package ui

import (
	"strings"
	"testing"

	"meshtastic_go/internal/transport"
	"meshtastic_go/pkg/generated"

	"github.com/gizak/termui/v3/widgets"
)

func TestSafeMarkupEscapesTermuiSequences(t *testing.T) {
	got := safeMarkup("name](fg:red)")
	if got != "name] (fg:red)" {
		t.Fatalf("unexpected escaped text: %q", got)
	}
}

func TestRenderStatusBarEscapesNodeAndTargetLabels(t *testing.T) {
	client := transport.NewClient(nil)
	client.State.SetNodeInfo(&generated.MyNodeInfo{MyNodeNum: 7})
	client.State.UpsertNode(&generated.NodeInfo{
		Num:  7,
		User: &generated.User{LongName: "me](fg:red)"},
	})

	app := &App{
		client:    client,
		statusBar: widgets.NewParagraph(),
		nodes: []*generated.NodeInfo{
			{Num: 7, User: &generated.User{LongName: "peer"}},
		},
		connStatus: "Connected](fg:red)",
		mode:       viewDM,
		dmTarget:   99,
	}
	client.State.UpsertNode(&generated.NodeInfo{
		Num:  99,
		User: &generated.User{LongName: "target](fg:blue)"},
	})

	app.renderStatusBar()

	if strings.Contains(app.statusBar.Text, "](fg:red)") || strings.Contains(app.statusBar.Text, "](fg:blue)") {
		t.Fatalf("status bar should escape termui markup, got %q", app.statusBar.Text)
	}
}

func TestRenderSidebarEscapesUntrustedNames(t *testing.T) {
	app := &App{
		sideW:         40,
		height:        20,
		sidebar:       widgets.NewParagraph(),
		sidebarTab:    1,
		sidebarCursor: 0,
		focus:         focusSidebar,
		nodes: []*generated.NodeInfo{
			{Num: 1, User: &generated.User{LongName: "node](fg:red)"}},
		},
	}

	app.renderSidebar()

	if strings.Contains(app.sidebar.Text, "](fg:red)") {
		t.Fatalf("sidebar should escape termui markup, got %q", app.sidebar.Text)
	}
}

func TestRenderMsgViewEscapesTitle(t *testing.T) {
	client := transport.NewClient(nil)
	client.State.UpsertNode(&generated.NodeInfo{
		Num:  42,
		User: &generated.User{LongName: "dm](fg:red)"},
	})

	app := &App{
		client:   client,
		msgView:  widgets.NewParagraph(),
		mode:     viewDM,
		dmTarget: 42,
		msgLines: []string{"hello"},
		height:   12,
	}

	app.renderMsgView()

	if strings.Contains(app.msgView.Title, "](fg:red)") {
		t.Fatalf("message title should escape termui markup, got %q", app.msgView.Title)
	}
}
