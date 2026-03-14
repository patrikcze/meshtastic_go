# AGENTS.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Project Overview

A Go TUI application for interacting with Meshtastic LoRa mesh networking devices over serial connections. Built with termui (gizak/termui) for the terminal UI and protobuf for the Meshtastic device protocol.

## Build & Development Commands

```bash
# Build all platform targets (windows/amd64, linux/amd64, linux/arm, darwin/arm64)
make build

# Build + lint
make

# Lint (uses golint, gosec, go vet)
make lint

# Clean build artifacts
make clean

# Run locally (macOS ARM)
./bin/meshtastic_go_darwin_arm64

# Run tests
go test ./...

# Run a single test
go test ./internal/transport/ -run TestFunctionName

# Vet
go vet ./...

# Trunk linting (alternative to make lint)
trunk check
```

The Makefile injects the git version tag via `-ldflags="-X main.version=$(VERSION)"` and builds with `-tags netgo`.

## Architecture

### Layer Diagram

```
cmd/main.go          — Entry point: wires serial → transport → protocol → UI
    ↓
pkg/serial/          — Device discovery (USB VID/PID matching) and serial port connection
    ↓
internal/transport/  — Protocol framing (StreamConn), Client state machine, event dispatch
    ↓
internal/protocol/   — Protobuf message handling: routing FromRadio variants, sending ToRadio
    ↓
internal/ui/         — termui TUI with sidebar (channels/nodes), message viewport, text input
    ↓
pkg/generated/       — Auto-generated protobuf Go code (DO NOT edit manually)
```

### Key Concepts

**Connection lifecycle** (`cmd/main.go`):
1. `serial.GetPorts()` discovers Meshtastic USB devices by VID/PID and OS-specific port patterns
2. `serial.Connect()` opens the serial port at 115200 baud
3. `transport.NewRadioStreamConn()` wraps the port with the Meshtastic stream framing protocol (magic bytes `0x94 0xc3` + length-prefixed protobuf)
4. `transport.NewClient()` manages connection state; `Client.Connect(ctx)` sends `WantConfigId` and blocks until `ConfigCompleteId` is received
5. A read goroutine continuously reads `FromRadio` messages and routes them via `protocol.HandleMessageProto()`

**StreamConn** (`internal/transport/stream_conn.go`): Implements the Meshtastic streaming protocol over `io.ReadWriteCloser`. Two constructors exist — `NewClientStreamConn` (sends wake bytes first) and `NewRadioStreamConn` (no wake). `PacketMTU` is 512 bytes.

**Client State** (`internal/transport/client.go`): Thread-safe `State` struct accumulates `MyNodeInfo`, `DeviceMetadata`, `NodeInfo[]`, `Channel[]`, `Config[]`, `ModuleConfig[]` during the initial config handshake. All accessors clone via `proto.Clone()`.

**Handler Registry** (`internal/transport/handlers.go`): Maps protobuf message type names (via `proto.MessageName`) to handler functions. Handlers run in goroutines.

**Event Dispatcher** (`internal/transport/dispatcher.go`): A separate pub/sub system keyed by string event types (e.g., `"MeshPacketReceived"`). Event handlers also run in goroutines.

**Protocol layer** (`internal/protocol/`): `HandleMessageProto` is the central router — switches on `FromRadio` payload variant types and delegates to specialized handlers or dispatches events. `sender.go` constructs `ToRadio` messages for sending text.

**UI** (`internal/ui/model.go`): termui `App` with five Paragraph widgets — status bar, sidebar, message viewport, text input, and help bar. Uses a `select`-based event loop reading from both `tui.PollEvents()` and `Client.Events`. Uses `internal/ui/store.MessageStore` for thread-safe message storage.

### Protobuf / Generated Code

`pkg/generated/` contains `.pb.go` files generated from Meshtastic protobuf definitions. These are checked in and should not be edited by hand. Key types: `FromRadio`, `ToRadio`, `MeshPacket`, `Data`, `Channel`, `Config`, `NodeInfo`.

## Linting

The project uses **Trunk** (`.trunk/trunk.yaml`) with:
- `gofmt` for formatting
- `golangci-lint` for Go linting
- `osv-scanner` for dependency vulnerability scanning
- `trufflehog` for secret detection

The Makefile also runs `golint`, `gosec`, and `go vet` via `make lint`.

## Important Patterns

- All protobuf state accessors in `State` use `proto.Clone()` to return safe copies — maintain this pattern when adding new state fields.
- Handler functions (both `MessageHandler` and `EventHandler`) are dispatched in goroutines — they must be goroutine-safe.
- Serial port detection is cross-platform with OS-specific logic in `pkg/serial/serial.go` and known USB VID/PIDs in `pkg/serial/usb.go`.
- The project uses two separate message dispatch systems: `HandlerRegistry` (protobuf-typed, for `Client`) and `EventDispatcher` (string-typed, for protocol layer). Don't conflate them.
