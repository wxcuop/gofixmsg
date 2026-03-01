# Copilot Instructions for GoFixMsg

## Build, Test, and Lint

GoFixMsg is a production-grade Go implementation of the FIX (Financial Information Exchange) protocol with session management, persistence, and comprehensive testing.

**Install dependencies:**
```bash
cd gofixmsg && go mod tidy
```

**Build:**
```bash
cd gofixmsg && go build ./...
```

**Run all tests:**
```bash
cd gofixmsg && go test ./... -v
```

**Run tests with race detection** (recommended for concurrency validation):
```bash
cd gofixmsg && go test -race ./... -v -timeout 10m
```

**Run tests with coverage:**
```bash
cd gofixmsg && go test ./... -v -race -coverprofile=coverage.out -covermode=atomic
```

**Run specific package tests:**
```bash
cd gofixmsg && go test ./engine -v
cd gofixmsg && go test ./integration -v
cd gofixmsg && go test -run TestApplicationCallbacks ./integration -v
```

**Lint and format:**
```bash
cd gofixmsg && go fmt ./...
cd gofixmsg && go vet ./...
```

## Architecture Overview

GoFixMsg consists of two main usage modes:

### 1. Parser-Only Mode
Use GoFixMsg as a lightweight FIX message parser without session management.

**Components:**
- `fixmsg/` — In-memory FIX message representation with field access
- `fixmsg/codec/` — Wire-format parsing and serialization
- `fixmsg/spec/` — QuickFIX XML specification loading for repeating group support
- `cmd/parse/` — CLI tool for parsing FIX messages

**Example:**
```go
import "github.com/wxcuop/gofixmsg/fixmsg"

msg := fixmsg.NewFixMessage()
msg.Set(35, "D")  // MsgType = NewOrderSingle
msg.Set(55, "AAPL") // Symbol
wire, _ := msg.ToWire()
```

### 2. Full Session Engine Mode
Use GoFixMsg for production FIX sessions with heartbeat, sequence management, persistence, and state machine.

**Core Components:**

| Package | Component | Role |
|---------|-----------|------|
| `engine/` | `FixEngine` | Central session coordinator. Manages initiator/acceptor modes, message routing, and lifecycle |
| `engine/session/` | `Session` | Per-connection read/write loops with sequential message processing |
| `engine/handler/` | `MessageProcessor`, `MessageHandler` | Registered handlers by FIX MsgType (e.g., 'A'=Logon, 'D'=NewOrder) |
| `state/` | `StateMachine` | Session state transitions: Disconnected → Connecting → AwaitingLogon → Active → ... |
| `heartbeat/` | `Heartbeat` | Periodic heartbeat ticker with test request tracking |
| `network/` | `Initiator`, `Acceptor` | TCP/TLS socket management |
| `store/` | `SQLiteStore` | Thread-safe SQLite persistence of sent/received messages |
| `scheduler/` | `RuntimeScheduler` | Time-based action scheduling (e.g., dailyLogon at 08:00) |

**Example:**
```go
import "github.com/wxcuop/gofixmsg/engine"

engine := engine.NewFixEngine(initiator)
engine.SetApplication(myApp)  // Implement callbacks
engine.SetupComponents(stateMachine, store)
engine.Connect()  // Blocks until connected
defer engine.Close()
```

## Key Conventions

### Concurrency Model
GoFixMsg uses goroutines for true concurrent execution:
- **Session.readLoop()**: Goroutine that reads from socket and processes incoming messages sequentially
- **Session.writeLoop()**: Goroutine that drains the send queue to the socket
- **Heartbeat goroutines**: Ticker for periodic heartbeat/test request

All shared state is protected by `sync.Mutex`. See [DEVELOPER.md](../gofixmsg/DEVELOPER.md) for synchronization patterns and resource cleanup best practices.

### Application Interface
Implement `engine.Application` to receive FIX callbacks:
```go
type MyApplication struct{}

func (a *MyApplication) OnCreate(sessionID string) { }
func (a *MyApplication) OnLogon(sessionID string) { }
func (a *MyApplication) OnLogout(sessionID string) { }
func (a *MyApplication) ToAdmin(msg *fixmsg.FixMessage, sessionID string) error { return nil }
func (a *MyApplication) FromAdmin(msg *fixmsg.FixMessage, sessionID string) error { return nil }
func (a *MyApplication) ToApp(msg *fixmsg.FixMessage, sessionID string) error { return nil }
func (a *MyApplication) FromApp(msg *fixmsg.FixMessage, sessionID string) error { return nil }
func (a *MyApplication) OnMessage(msg *fixmsg.FixMessage, sessionID string) { }
func (a *MyApplication) OnReject(msg *fixmsg.FixMessage, reason string, sessionID string) { }
```

### Message Handler Registration
Register handlers by FIX MsgType code:
```go
processor := handler.NewProcessor()
processor.Register("D", func(msg *fixmsg.FixMessage) error {
    // Handle NewOrderSingle
    return nil
})
```

### FIX Tag Access
All tags are integers (see `fixmsg/tags.go` for constants):
```go
msgType, _ := msg.Get(35)           // MsgType (2-valued return)
clOrdID := msg.MustGet(11)          // ClOrdID (panics if missing)
msg.Set(55, "AAPL")                 // Set Symbol
```

### Session ID Format
Session IDs are formatted as `"{SENDER}-{TARGET}-{HOST}:{PORT}"`:
```go
sessionID := fmt.Sprintf("%s-%s-%s:%d", senderID, targetID, host, port)
```

### Data Race Prevention
Go requires explicit mutex synchronization (no `synchronized` keyword like Java):
```go
// Always protect shared state
t.mu.Lock()
defer t.mu.Unlock()
*t.callOrder = append(*t.callOrder, ...)
```

See [DEVELOPER.md](../gofixmsg/DEVELOPER.md) for:
- Critical synchronization points (FixEngine.attachMu, Heartbeat.mu, Session.mu)
- Initialization order principles
- How to use `go test -race ./...` for validation
- Resource cleanup patterns and timeout-wrapped cleanup best practices
