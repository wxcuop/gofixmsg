# GoFixMsg Developer Guide

This guide provides a comprehensive overview of GoFixMsg architecture, components, and how to use them to build FIX applications.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Core Components](#core-components)
3. [Using GoFixMsg as a Message Parser](#using-gofixmsg-as-a-message-parser)
4. [Using GoFixMsg as a Session Engine](#using-gofixmsg-as-a-session-engine)
5. [FIX Specification Loading](#fix-specification-loading)
6. [Message Store and Persistence](#message-store-and-persistence)
7. [Concurrency and Threading](#concurrency-and-threading)
8. [Best Practices](#best-practices)
9. [Examples and Debugging](#examples-and-debugging)

---

## Architecture Overview

GoFixMsg is a dual-purpose FIX library:

1. **FIX Message Parser/Creator** - Parse FIX messages from wire format, manipulate fields, serialize back to wire format
2. **FIX Session Engine** - Manage FIX sessions including logon, heartbeat, sequence number management, and message persistence

### Two Usage Modes

| Mode | Use Case | Components | Complexity |
|------|----------|-----------|-----------|
| **Parser-Only** | CLI tools, message validation, ad-hoc parsing | `fixmsg` package | Low |
| **Full Session** | Production trading systems, gateways, bridges | `fixmsg` + `engine` + `network` + `store` | High |

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Application Layer                            │
│                  (Your FIX application)                          │
└────────────────────┬────────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────────┐
│                    FixEngine (engine/)                           │
│  • Session management    • Reconnection logic                    │
│  • Message handlers      • Heartbeat monitoring                  │
│  • Sequence management   • State transitions                     │
└────────────────┬──────────────────────┬────────────────────────┘
                 │                      │
        ┌────────▼──────────┐  ┌─────────▼────────────┐
        │ StateMachine      │  │ MessageProcessor     │
        │ (state/)          │  │ (handler/)           │
        └───────────────────┘  └──────────────────────┘
                 │                      │
        ┌────────▼──────────┐  ┌─────────▼────────────┐
        │ Network           │  │ FixMessage           │
        │ (network/)        │  │ (fixmsg/)            │
        │ • TCP/TLS         │  │ • Parse/Serialize    │
        │ • Initiator       │  │ • Field Access       │
        │ • Acceptor        │  │ • Codec Support      │
        └───────────────────┘  └──────────────────────┘
                 │
        ┌────────▼──────────┐
        │ Message Store     │
        │ (store/)          │
        │ • SQLite DB       │
        │ • Persistence     │
        └───────────────────┘
```

---

## Core Components

### 1. FixMessage (`fixmsg/` package)

**What it is:** In-memory representation of a FIX message.

**Key types:**
```go
// FixFragment is a map of tag (int) → value (string)
type FixFragment map[int]string

// FixMessage extends FixFragment with metadata
type FixMessage struct {
    FixFragment              // Tag map
    Codec      fixmsg.Codec  // Wire format handler
    Time       time.Time     // Creation/receive time
    Direction  int           // 0=inbound, 1=outbound
    RawMessage []byte        // Original wire bytes
}
```

**Core operations:**
```go
// Create
msg := fixmsg.NewFixMessage()
msg = fixmsg.NewFixMessageFromMap(map[int]string{35: "D", 55: "AAPL"})

// Field access
msg.Set(55, "AAPL")                    // Set field
value, exists := msg.Get(55)           // Get field
mustValue := msg.MustGet(55)           // Get or panic

// Wire format
wire, _ := msg.ToWire()                // Serialize (auto sets len/checksum)
msg.LoadFix(wire)                      // Parse from wire
msg.SetLenAndChecksum()                // Manual checksum calculation
```

**Comparisons:**
```go
msg.TagExact(44, "150.25", false)      // Exact match
msg.TagContains(55, "AAPL", false)     // Contains substring
```

### 2. FixSpec (`fixmsg/spec/` package)

**What it is:** QuickFIX XML specification loaded into memory.

**Provides:**
- Message type definitions (which fields are required)
- Field metadata (type, enum values)
- Repeating group specifications
- Tag name lookups

**Loading:**
```go
import "github.com/wxcuop/gofixmsg/fixmsg/spec"

// From file
s, err := spec.Load("FIX44.xml")    // 6,599 lines; 309 KB

// From bytes (tests)
s, err := spec.LoadBytes([]byte(`<fix ...>...</fix>`))
```

### 3. Codec (`fixmsg/codec/` package)

**What it is:** Bridge between in-memory `FixMessage` and wire format bytes.

**Two variants:**
```go
// Without spec (no repeating group support)
c := codec.NewNoGroups()

// With spec (full repeating group parsing)
s, _ := spec.Load("FIX44.xml")
c := codec.New(s)
```

**Operations:**
```go
// Parse from wire format
msg, err := c.Parse(wireBytes)

// Serialize to wire
msg.Codec = c               // Attach codec
wire, err := msg.ToWire()   // Uses codec.Serialise()
```

### 4. FixEngine (`engine/` package)

**What it is:** Central coordinator for FIX session management.

**Responsibilities:**
- Session logon/logout
- Message processing and routing
- Sequence number management
- Heartbeat monitoring
- Reconnection with backoff
- Application callbacks

**Lifecycle:**
```go
// Create
engine := engine.NewFixEngine(network.NewInitiator("host:port"))

// Configure
engine.SetApplication(myApp)

// Setup components
engine.SetupComponents(state.NewStateMachine(), store.NewSQLiteStore())

// Connect
engine.Connect()

// Send messages
engine.SendMessage(msg)

// Shutdown
engine.Close()
```

### 5. Application Callback Interface (`engine/application.go`)

**What it is:** Interface your application implements to handle FIX events.

**Required methods:**
```go
type Application interface {
    // Session lifecycle
    OnCreate(sessionID string)
    OnLogon(sessionID string)
    OnLogout(sessionID string)

    // Admin message processing (Logon, Heartbeat, etc.)
    ToAdmin(msg *fixmsg.FixMessage, sessionID string) error
    FromAdmin(msg *fixmsg.FixMessage, sessionID string) error

    // Application message processing (NewOrder, ExecutionReport, etc.)
    ToApp(msg *fixmsg.FixMessage, sessionID string) error
    FromApp(msg *fixmsg.FixMessage, sessionID string) error

    // Generic message received
    OnMessage(msg *fixmsg.FixMessage, sessionID string)
}
```

### 6. StateMachine (`state/` package)

**What it is:** FIX session state manager.

**States:**
```
Disconnected → Connecting → AwaitingLogon → LoggedOn → (processing) → LoggingOut → Disconnected
```

**Subscribers:**
```go
sm := state.NewStateMachine()
changes := sm.Subscribe()  // Channel of state change strings
go func() {
    for state := range changes {
        fmt.Printf("State changed to: %s\n", state)
    }
}()
```

### 7. NetworkConnection (`network/` package)

**What it is:** TCP/TLS connection handler.

**Two modes:**
- **Initiator**: Connects to remote peer
- **Acceptor**: Listens for incoming connections

**Usage:**
```go
// Initiator
init := network.NewInitiator("server.example.com:5001")
init.WithTLS(tlsCfg)  // Optional

// Acceptor
acc := network.NewAcceptor("0.0.0.0:5001")
acc.Start(func(conn *network.Conn) {
    // Handle new connection
})
```

### 8. MessageStore (`store/` package)

**What it is:** Persistent storage for FIX messages and session state.

**Interface:**
```go
type Store interface {
    Init(path string) error
    SaveMessage(m *Message) error
    GetMessage(begin, sender, target string, seq int) (*Message, error)
    SaveSessionSeq(sessionID string, outSeq, inSeq int) error
    GetSessionSeq(sessionID string) (outSeq, inSeq int, err error)
}
```

**Implementation:** SQLite with WAL mode for concurrent access.

---

## Using GoFixMsg as a Message Parser

### Simple Message Parsing

```go
import "github.com/wxcuop/gofixmsg/fixmsg/codec"

// Raw FIX message with SOH (0x01) delimiters
wireData := []byte("8=FIX.4.4\x019=50\x0135=A\x0149=SENDER\x0156=TARGET\x01108=30\x0110=123\x01")

// Parse without spec
codec := codec.NewNoGroups()
msg, err := codec.Parse(wireData)
if err != nil {
    log.Fatal(err)
}

// Access fields
fmt.Printf("MsgType: %s\n", msg.Get(35))
fmt.Printf("Sender: %s\n", msg.Get(49))
fmt.Printf("HeartBtInt: %s\n", msg.Get(108))

// Iterate all fields
for tag, value := range msg.FixFragment {
    fmt.Printf("Tag %d: %s\n", tag, value)
}
```

### Message Parsing with Spec Support

```go
import "github.com/wxcuop/gofixmsg/fixmsg/spec"

// Load spec for repeating group support
s, err := spec.Load("FIX44.xml")
if err != nil {
    log.Fatal(err)
}

// Create codec with spec
c := codec.New(s)

// Parse message
msg, err := c.Parse(wireData)
if err != nil {
    log.Fatal(err)
}

// Now repeating groups are properly parsed
if group, exists := msg.GetGroup(268); exists {  // NoMDEntries
    for i := 0; i < group.Len(); i++ {
        entry := group.At(i)
        fmt.Printf("Entry %d: %s\n", i, entry.Get(270))  // MDEntryPx
    }
}
```

### Creating FIX Messages Programmatically

```go
// Create new message
msg := fixmsg.NewFixMessage()

// Set standard header
msg.Set(fixmsg.TagBeginString, "FIX.4.4")
msg.Set(fixmsg.TagMsgType, "D")          // NewOrderSingle
msg.Set(fixmsg.TagSenderCompID, "TRADER")
msg.Set(fixmsg.TagTargetCompID, "EXCHANGE")
msg.Set(fixmsg.TagMsgSeqNum, "1")

// Set application fields
msg.Set(11, "ORDER-001")                 // ClOrdID
msg.Set(55, "AAPL")                      // Symbol
msg.Set(54, "1")                         // Side (1=Buy)
msg.Set(38, "1000")                      // OrderQty
msg.Set(40, "2")                         // OrdType (2=Limit)
msg.Set(44, "150.25")                    // Price

// Serialize to wire format
wire, err := msg.ToWire()
if err != nil {
    log.Fatal(err)
}

// Send or store
fmt.Printf("Wire length: %d bytes\n", len(wire))
```

### Field Type Handling

**Important:** All fields in FixMessage are stored as strings.

```go
// String values
msg.Set(55, "AAPL")                      // Symbol
msg.Set(54, "1")                         // Side

// Numeric values (stored as strings)
msg.Set(38, "1000")                      // OrderQty
msg.Set(44, "150.25")                    // Price

// Type conversions (your responsibility)
qtyStr, _ := msg.Get(38)
qty, _ := strconv.Atoi(qtyStr)           // Convert to int

priceStr, _ := msg.Get(44)
price, _ := strconv.ParseFloat(priceStr) // Convert to float
```

### Message Comparison and Searching

```go
// Exact match
if msg.TagExact(54, "1", false) {        // Side
    fmt.Println("Buy order")
}

// Case-insensitive comparison
if msg.TagExact(55, "aapl", true) {  
    fmt.Println("Apple stock")
}

// Contains substring
if msg.TagContains(55, "PL", false) {
    fmt.Println("Symbol contains 'PL'")
}

// Find all occurrences of a tag (in nested repeating groups)
paths := msg.FindAll(270)                 // MDEntryPx
// Returns: [[268, 0, 270], [268, 1, 270]]  for group members
```

---

## Using GoFixMsg as a Session Engine

### Initiator (Client) Setup

```go
import (
    "github.com/wxcuop/gofixmsg/engine"
    "github.com/wxcuop/gofixmsg/network"
    "github.com/wxcuop/gofixmsg/state"
    "github.com/wxcuop/gofixmsg/store"
)

// Implement application callbacks
type MyApp struct {
    engine.NoOpApplication
}

func (a *MyApp) OnLogon(sessionID string) {
    fmt.Printf("Logon successful: %s\n", sessionID)
}

func (a *MyApp) FromApp(msg *fixmsg.FixMessage, sessionID string) error {
    fmt.Printf("Received app message type %s\n", msg.Get(35))
    return nil
}

func main() {
    // Create network initiator
    initiator := network.NewInitiator("server.example.com:5001")

    // Create engine
    fe := engine.NewFixEngine(initiator)
    fe.SetApplication(&MyApp{})

    // Create state machine
    sm := state.NewStateMachine()

    // Create message store
    msgStore := store.NewSQLiteStore()
    if err := msgStore.Init("fix_messages.db"); err != nil {
        log.Fatal(err)
    }

    // Wire everything together
    fe.SetupComponents(sm, msgStore)

    // Connect (blocking until Close())
    if err := fe.Connect(); err != nil {
        log.Fatal(err)
    }
}
```

### Acceptor (Server) Setup

```go
// Create multi-session acceptor
acceptor := engine.NewMultiSessionEngine("0.0.0.0:5001")

// Register application
acceptor.SetApplication(&MyApp{})

// Start listening
if err := acceptor.Start(); err != nil {
    log.Fatal(err)
}

// Block until stop signal
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
<-sigChan

acceptor.Stop()
```

### Sending Messages

```go
// Build message
order := fixmsg.NewFixMessage()
order.Set(fixmsg.TagBeginString, "FIX.4.4")
order.Set(fixmsg.TagMsgType, "D")
order.Set(11, "ORDER-123")
order.Set(55, "AAPL")
order.Set(54, "1")
order.Set(38, "100")
order.Set(40, "2")
order.Set(44, "150.00")

// Send
if err := fe.SendMessage(order); err != nil {
    log.Printf("Failed to send: %v", err)
}
```

### Sequence Number Management

**Automatic:** Handled by engine via SeqManager.

**Manual access:**
```go
// Get current outbound sequence number
outSeq := fe.GetSeqMgr().GetNextOutSeqNum(sessionID)

// Session persists sequence numbers across restarts via message store
```

### Session State Monitoring

```go
sm := state.NewStateMachine()
stateChan := sm.Subscribe()

go func() {
    for state := range stateChan {
        fmt.Printf("Session state: %s\n", state)
        // Possible values: "Disconnected", "Connecting", "AwaitingLogon", "LoggedOn", etc.
    }
}()

fe.SetupComponents(sm, msgStore)
```

---

## FIX Specification Loading

### Why Load a Spec?

Without spec:
- ✗ Repeating groups parsed as regular tags
- ✗ No message type validation
- ✗ No field metadata
- ✓ Faster loading
- ✓ Simpler code

With spec:
- ✓ Repeating groups properly parsed
- ✓ Message type definitions
- ✓ Field validation
- ✓ Enum values
- ✗ Slower parsing
- ✗ Large file (309 KB for FIX44)

### Loading Spec from File

```go
import "github.com/wxcuop/gofixmsg/fixmsg/spec"

// Load from file
s, err := spec.Load("FIX44.xml")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Loaded spec version: %s\n", s.Version)  // FIX.4.4

// Create codec with spec
c := codec.New(s)

// Use for parsing
msg, _ := c.Parse(wireData)
```

### Spec File Locations

The examples automatically search for specs:

```go
func findSpecFile(filename string) string {
    searchPaths := []string{
        filename,                              // Current dir
        filepath.Join("..", filename),         // Parent dir
        filepath.Join("..", "..", filename),   // Two levels up
    }
    
    for _, path := range searchPaths {
        if info, err := os.Stat(path); err == nil {
            return path
        }
    }
    return ""
}

specFile := findSpecFile("FIX44.xml")
if specFile != "" {
    s, _ := spec.Load(specFile)
    // Use spec
} else {
    // Fall back to basic codec
    c := codec.NewNoGroups()
}
```

### Getting Spec from QuickFIX

Official specs: https://github.com/quickfix/quickfix/tree/master/spec

```bash
# Download FIX 4.4 spec
curl -O https://raw.githubusercontent.com/quickfix/quickfix/master/spec/FIX44.xml

# Download other versions
curl -O https://raw.githubusercontent.com/quickfix/quickfix/master/spec/FIX42.xml
curl -O https://raw.githubusercontent.com/quickfix/quickfix/master/spec/FIX50.xml
```

---

## Message Store and Persistence

### SQLite Message Store

**Purpose:**
- Persist FIX messages for gap filling on reconnection
- Store session sequence numbers
- Enable recovery from application restarts

**Setup:**

```go
import "github.com/wxcuop/gofixmsg/store"

// Create and initialize store
msgStore := store.NewSQLiteStore()

if err := msgStore.Init("fix_sessions.db"); err != nil {
    log.Fatal(err)
}

// Wire into engine
fe.SetupComponents(state.NewStateMachine(), msgStore)
```

### Database Schema

```sql
-- Messages table (primary key is composite)
CREATE TABLE messages (
    beginstring TEXT NOT NULL,
    sendercompid TEXT NOT NULL,
    targetcompid TEXT NOT NULL,
    msgseqnum INTEGER NOT NULL,
    msgtype TEXT,
    body BLOB,
    created INTEGER,
    PRIMARY KEY (beginstring, sendercompid, targetcompid, msgseqnum)
);

-- Sessions table
CREATE TABLE sessions (
    sessionid TEXT PRIMARY KEY,
    seq INTEGER NOT NULL,           -- Outbound sequence number
    in_seq INTEGER NOT NULL         -- Inbound sequence number
);
```

### SQLite Pragmas

```sql
-- Write-Ahead Logging mode (better concurrent read performance)
PRAGMA journal_mode=WAL;

-- Balanced sync strategy
PRAGMA synchronous=NORMAL;
```

### Message Store API

**Saving messages:**
```go
msg := &store.Message{
    BeginString:  "FIX.4.4",
    SenderCompID: "CLIENT",
    TargetCompID: "SERVER",
    MsgSeqNum:    42,
    MsgType:      "D",
    Body:         wireBytes,
    Created:      time.Now(),
}

if err := msgStore.SaveMessage(msg); err != nil {
    log.Printf("Save failed: %v", err)
}
```

**Retrieving messages:**
```go
msg, err := msgStore.GetMessage("FIX.4.4", "CLIENT", "SERVER", 42)
if err != nil {
    log.Printf("Get failed: %v", err)
}
if msg != nil {
    fmt.Printf("Found message type %s\n", msg.MsgType)
}
```

**Session state:**
```go
// Save
if err := msgStore.SaveSessionSeq("CLIENT-SERVER-127.0.0.1:5001", 43, 51); err != nil {
    log.Fatal(err)
}

// Retrieve
outSeq, inSeq, err := msgStore.GetSessionSeq("CLIENT-SERVER-127.0.0.1:5001")
```

### Performance Characteristics

- **Write:** ~1-2ms per message
- **Read:** ~0.5-1ms per message  
- **Concurrent access:** Safe via SQLite driver serialization + WAL
- **Disk per message:** ~300 bytes

### Maintenance

**Cleanup old messages:**
```go
import "database/sql"
_ "modernc.org/sqlite"

db, _ := sql.Open("sqlite", "fix_sessions.db")
defer db.Close()

// Delete messages older than 30 days
_, err := db.Exec(`
    DELETE FROM messages 
    WHERE created < datetime('now', '-30 days')
`)
```

**Database inspection:**
```bash
sqlite3 fix_sessions.db

> SELECT COUNT(*) FROM messages;
42

> SELECT * FROM sessions;
CLIENT-SERVER-127.0.0.1:5001|43|51

> .schema messages
CREATE TABLE messages(...)
```

---

## Concurrency and Threading

### SQLite Threading Model

**Challenge:** SQLite is single-threaded.

**Solution:** `modernc.org/sqlite` driver handles serialization internally.

**Result:** Multiple goroutines can safely call `msgStore.SaveMessage()` and `msgStore.GetMessage()` simultaneously—operations are internally queued.

```go
// Safe for concurrent use
go func() {
    for i := 0; i < 100; i++ {
        msgStore.SaveMessage(msg1)
    }
}()

go func() {
    for i := 0; i < 100; i++ {
        msgStore.SaveMessage(msg2)
    }
}()

go func() {
    for i := 0; i < 100; i++ {
        msgStore.GetMessage(...)
    }
}()

// All goroutines can run simultaneously; database access is serialized internally
```

### Goroutine Patterns in GoFixMsg

**Message reading loop (per session):**
```go
// Inside FixEngine
go func() {
    for {
        msg, err := readFromNetwork()
        if err != nil {
            break
        }
        engine.HandleIncoming(msg)
    }
}()
```

**Heartbeat monitor:**
```go
// Sends periodic heartbeats
go func() {
    ticker := time.NewTicker(heartbeatInterval)
    defer ticker.Stop()
    for range ticker.C {
        engine.SendHeartbeat()
    }
}()
```

**Multi-session acceptor:**
```go
// Each connection gets its own engine with separate goroutines
acceptor.Start(func(conn *network.Conn) {
    engine := createNewEngine(conn)
    engine.Connect()  // Blocks with read loop
})
```

### Data Race Prevention

**Critical synchronization points:**

1. **FixEngine.AttachSession/DetachSession (`engine/attach.go`)**
   - Protected by `attachMu sync.Mutex`
   - Ensures atomic initialization of Monitor and hbSender before session starts
   - Prevents race when callbacks execute on parallel goroutine

2. **Heartbeat.Start/Stop (`heartbeat/heartbeat.go`)**
   - Protected by `mu sync.Mutex`
   - Synchronizes access to `cancel` context handle
   - Prevents concurrent start/stop races

3. **Test integration callbacks (`integration/application_callbacks_test.go`)**
   - Application callback implementations must protect shared state with mutex
   - Test assertion functions must acquire mutex before reading callback order

**Key initialization order principle:**
```go
// CORRECT: Monitors initialized BEFORE session starts
e.Monitor = NewHeartbeatMonitor(...)
e.hbSender = heartbeat.New(...)
s.Start()  // Now safe for callbacks to access Monitor/hbSender

// WRONG: Monitors initialized after session starts
s.Start()
e.Monitor = NewHeartbeatMonitor(...)  // Race condition!
```

**Validating data races:**
```bash
# Run test suite with race detector enabled
go test -race ./...

# Zero data races should be detected
```

### Thread Safety Guarantees

✓ FixMessage is thread-safe for concurrent reads  
✓ FixMessage is NOT thread-safe for concurrent writes (use mutex if needed)  
✓ FixEngine operations are thread-safe (attachMu protects lifecycle)  
✓ Heartbeat is thread-safe (mu protects start/stop lifecycle)  
✓ StateMachine is thread-safe  
✓ MessageStore is thread-safe  

---

## Best Practices

### 1. Always Load FIX Spec for Production

```go
// Good: Production ready
s, _ := spec.Load("FIX44.xml")
c := codec.New(s)

// Risky: Limited repeating group support
c := codec.NewNoGroups()
```

### 2. Implement Proper Error Handling

```go
// Not just logging
if err := msgStore.SaveMessage(msg); err != nil {
    log.Fatal(err)  // Don't ignore persistence errors
}

// Implement application-level retries
msg := fixmsg.NewFixMessage()
msg.Set(...)
retries := 3
for i := 0; i < retries; i++ {
    if err := fe.SendMessage(msg); err == nil {
        break
    }
    time.Sleep(time.Second * time.Duration(i+1))
}
```

### 3. Use Session IDs Consistently

**Format:** `"{SENDER}-{TARGET}-{HOST}:{PORT}"`

```go
sessionID := fmt.Sprintf("%s-%s-%s:%d", senderID, targetID, host, port)

// Use in callbacks
func (a *MyApp) OnLogon(sessionID string) {
    fmt.Printf("Logon: %s\n", sessionID)  // CLIENT-SERVER-127.0.0.1:5001
}
```

### 4. Monitor Session State

```go
sm := state.NewStateMachine()
stateChan := sm.Subscribe()

go func() {
    for state := range stateChan {
        log.Printf("Session state: %s", state)
        // Handle transitions: send emails, update dashboards, etc.
    }
}()
```

### 5. Separate Parsing from Session Logic

```go
// Standalone parsing (no engine needed)
func parseMessage(wireBytes []byte) (*fixmsg.FixMessage, error) {
    c := codec.NewNoGroups()
    return c.Parse(wireBytes)  // Quick, lightweight
}

// Session-based processing (needs engine)
func processMessage(msg *fixmsg.FixMessage, engine *engine.FixEngine) error {
    engine.SendMessage(msg)  // Full session context
    return nil
}
```

### 6. Handle Repeating Groups

```go
// Check for repeating group
if group, exists := msg.GetGroup(268); exists {  // NoMDEntries
    for i := 0; i < group.Len(); i++ {
        entry := group.At(i)
        price, _ := entry.Get(270)      // MDEntryPx
        fmt.Printf("Entry %d price: %s\n", i, price)
    }
}

// Create repeating group
group := fixmsg.NewRepeatingGroup(268)
group.FirstTag = 279

member := group.Add()
member.Set(279, "0")
member.Set(270, "1.37215")

msg.SetGroup(268, group)
```

### 7. Graceful Shutdown

```go
// Handle OS signals
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigChan
    fmt.Println("Shutting down...")
    
    fe.Close()  // Close engine (sends logout)
    msgStore.Close()  // Close database
    
    os.Exit(0)
}()

// Or defer cleanup
defer func() {
    fe.Close()
    msgStore.Close()
}()
```

---

## Examples and Debugging

### Running Examples

```bash
# Parser-only examples
cd gofixmsg/examples/parser
go run main.go

# Comprehensive message manipulation (mirrors pyfixmsg)
cd gofixmsg/examples/comprehensive
go run main.go

# Advanced spec and serialization
cd gofixmsg/examples/specs
go run main.go

# CLI tools
go run ../cmd/parse/main.go -input "8=FIX.4.4|..."
go run ../cmd/create/main.go -type D -fields "55=AAPL,54=1"

# Session examples
cd gofixmsg/examples/acceptor
go run main.go

# In another terminal
cd gofixmsg/examples/initiator
go run main.go
```

### Debugging Techniques

**Inspect wire format:**
```go
// Show raw bytes with readable characters
func displayWire(wire []byte) {
    for _, b := range wire {
        if b == 0x01 {
            fmt.Print("|")
        } else if b >= 32 && b <= 126 {
            fmt.Printf("%c", b)
        } else {
            fmt.Printf("<%02x>", b)
        }
    }
    fmt.Println()
}

displayWire(msgWireBytes)  // Output: 8=FIX.4.4|9=50|35=A|...
```

**Log all messages:**
```go
type DebugApp struct {
    engine.NoOpApplication
}

func (a *DebugApp) FromApp(msg *fixmsg.FixMessage, sessionID string) error {
    fmt.Printf("[%s] Received: ", sessionID)
    for tag := range msg.FixFragment {
        fmt.Printf("%d=%s ", tag, msg.MustGet(tag))
    }
    fmt.Println()
    return nil
}

func (a *DebugApp) ToApp(msg *fixmsg.FixMessage, sessionID string) error {
    fmt.Printf("[%s] Sending: ", sessionID)
    for tag := range msg.FixFragment {
        fmt.Printf("%d=%s ", tag, msg.MustGet(tag))
    }
    fmt.Println()
    return nil
}
```

**Inspect message store:**
```bash
sqlite3 fix_sessions.db

# List all stored messages
SELECT sessionid, msgseqnum, msgtype, created FROM messages ORDER BY created DESC;

# List session state
SELECT * FROM sessions;

# Count messages per session
SELECT sessionid, COUNT(*) as count FROM messages GROUP BY sessionid;

# Delete test data
DELETE FROM messages WHERE created < datetime('now', '-1 day');
DELETE FROM sessions;
```

**Enable detailed state transitions:**
```go
sm := state.NewStateMachine()
stateChan := sm.Subscribe()

go func() {
    for state := range stateChan {
        fmt.Printf("[%s] State change -> %s\n", 
            time.Now().Format("15:04:05.000"), state)
    }
}()
```

### Testing

```go
// Unit test: Parse a message
func TestParseLogon(t *testing.T) {
    wire := []byte("8=FIX.4.4\x019=50\x0135=A\x01...")
    c := codec.NewNoGroups()
    msg, err := c.Parse(wire)
    
    if err != nil {
        t.Fatalf("Parse failed: %v", err)
    }
    
    if msgType, _ := msg.Get(35); msgType != "A" {
        t.Errorf("Wrong message type: %s", msgType)
    }
}

// Integration test: Round-trip
func TestRoundtrip(t *testing.T) {
    original := fixmsg.NewFixMessage()
    original.Set(35, "D")
    original.Set(55, "AAPL")
    
    wire, _ := original.ToWire()
    parsed := fixmsg.NewFixMessage()
    parsed.LoadFix(wire)
    
    if original.MustGet(55) != parsed.MustGet(55) {
        t.Error("Roundtrip failed")
    }
}
```

---

## Quick Reference

### Common Tag Numbers

| Tag | Name | Example |
|-----|------|---------|
| 8 | BeginString | FIX.4.4 |
| 9 | BodyLength | 150 |
| 10 | CheckSum | 023 |
| 35 | MsgType | A, D, 8 |
| 34 | MsgSeqNum | 1, 2, 3... |
| 49 | SenderCompID | CLIENT |
| 56 | TargetCompID | SERVER |
| 108 | HeartBtInt | 30 |
| 11 | ClOrdID | ORDER-001 |
| 55 | Symbol | AAPL |
| 54 | Side | 1=Buy, 2=Sell |
| 38 | OrderQty | 1000 |
| 40 | OrdType | 1=Market, 2=Limit |
| 44 | Price | 150.25 |

### Importing GoFixMsg

```go
import (
    "github.com/wxcuop/gofixmsg/fixmsg"          // Message parsing
    "github.com/wxcuop/gofixmsg/fixmsg/codec"    // Codec
    "github.com/wxcuop/gofixmsg/fixmsg/spec"     // Spec loading
    "github.com/wxcuop/gofixmsg/engine"          // Session engine
    "github.com/wxcuop/gofixmsg/state"           // State machine
    "github.com/wxcuop/gofixmsg/network"         // Network I/O
    "github.com/wxcuop/gofixmsg/store"           // Message persistence
)
```

### Minimal Complete Example

```go
package main

import (
    "fmt"
    "log"
    "github.com/wxcuop/gofixmsg/fixmsg"
    "github.com/wxcuop/gofixmsg/fixmsg/codec"
)

func main() {
    // Parse a FIX message
    wire := []byte("8=FIX.4.4\x019=50\x0135=A\x0149=CLIENT\x0156=SERVER\x01108=30\x0110=023\x01")
    
    c := codec.NewNoGroups()
    msg, err := c.Parse(wire)
    if err != nil {
        log.Fatal(err)
    }
    
    // Access fields
    fmt.Printf("Message Type: %s\n", msg.MustGet(35))  // A
    fmt.Printf("Sender: %s\n", msg.MustGet(49))         // CLIENT
    fmt.Printf("Target: %s\n", msg.MustGet(56))         // SERVER
}
```

---

## See Also

- [Examples Directory](./examples/)
- [Migration from Python](./doc/migration.md)
- [API Parity with Python](./doc/API_PARITY.md)
- [QuickFIX Specifications](https://github.com/quickfix/quickfix/tree/master/spec)
