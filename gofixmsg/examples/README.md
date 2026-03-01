# GoFixMsg Examples

This directory contains runnable examples demonstrating how to build FIX applications using GoFixMsg.

## Overview

The examples show:
- **initiator/** - Connecting to a remote FIX peer as an Initiator
- **acceptor/** - Accepting connections from FIX peers as an Acceptor
- **parser/** - Library examples for parsing and creating FIX messages programmatically
- **comprehensive/** - Comprehensive examples covering message parsing, manipulation, copying, and repeating groups (mirrors Python pyfixmsg examples)
- **specs/** - Advanced examples for serialization, deserialization, field handling, and building complex messages

## Quick Start

### Parse a FIX Message (CLI Tool)

```bash
# Parse a message from command-line with | as SOH delimiter
go run ../cmd/parse/main.go -input "8=FIX.4.4|9=50|35=A|49=SENDER|56=TARGET|108=30|10=123"

# Parse with pretty output and tag names
go run ../cmd/parse/main.go -input "8=FIX.4.4|..." -pretty -tags

# Parse from a file
go run ../cmd/parse/main.go -file message.fix -pretty
```

### Create a FIX Message (CLI Tool)

```bash
# Create a simple Logon message
go run ../cmd/create/main.go -type A -sender TRADER -target EXCHANGE

# Create a New Order with additional fields (pretty output)
go run ../cmd/create/main.go -type D \
  -sender TRADER -target EXCHANGE \
  -fields "55=AAPL,54=1,40=2,38=1000,44=150.25" \
  -pretty -ascii

# Save to file
go run ../cmd/create/main.go -type D \
  -fields "55=AAPL,54=1,40=2,38=1000" \
  -output order.fix
```

### Library Examples

View full examples of using gofixmsg as a library:

```bash
cd parser
go run main.go
```

This demonstrates:
1. Creating messages programmatically
2. Parsing wire-format FIX messages
3. Accessing and modifying fields
4. Building complex messages
5. Field access patterns

## Prerequisites

- Go 1.21 or later
- FIX44.xml specification file (automatically downloaded; located at `gofixmsg/FIX44.xml`)
- A `config.ini` file (for session examples - see Configuration section)

## FIX Specification

The examples use the QuickFIX FIX 4.4 specification (FIX44.xml) for parsing and validating FIX messages.

**Location:** `gofixmsg/FIX44.xml` (309 KB)

The spec file is automatically searched in:
1. Current directory (`./FIX44.xml`)
2. Parent directory (`../FIX44.xml`)
3. Two levels up (`../../FIX44.xml`)

If the spec file is not found, examples gracefully degrade to basic parsing without repeating group support.

**Updating the Spec:**
To use a different FIX spec version, download from [QuickFIX](https://github.com/quickfix/quickfix/tree/master/spec) and replace `FIX44.xml`.

## Configuration

Both examples expect a `config.ini` file in this directory. Use the provided `config.ini` as a template:

```ini
[Session]
sender_comp_id = SENDER
target_comp_id = TARGET
heartbeat_interval = 30
host = 127.0.0.1
port = 5001
```

### Configuration Keys

| Key | Description | Example |
|-----|-------------|---------|
| `sender_comp_id` | Your company's identifier | `SENDER` |
| `target_comp_id` | Peer's identifier | `TARGET` |
| `heartbeat_interval` | Heartbeat timeout in seconds | `30` |
| `host` | For Initiator: peer's host; For Acceptor: bind address | `127.0.0.1` or `0.0.0.0` |
| `port` | Connection port | `5001` |
| `ssl_cert_file` | Path to TLS certificate (optional) | `/path/to/cert.pem` |
| `ssl_key_file` | Path to TLS private key (optional) | `/path/to/key.pem` |
| `ssl_ca_file` | Path to CA certificate for verification (optional) | `/path/to/ca.pem` |

## Running the Examples

### Acceptor

Start the acceptor (server) first:

```bash
cd acceptor
go run main.go
```

Expected output:
```
Starting FIX Acceptor on 0.0.0.0:5001...
Acceptor is running. Press Ctrl+C to stop.
```

### Initiator

In another terminal, start the initiator (client):

```bash
cd initiator
go run main.go
```

Expected output:
```
Connecting to FIX Acceptor on 127.0.0.1:5001...
Initiator is running. Press Ctrl+C to stop.
```

### Building

To build the examples as standalone binaries:

```bash
go build -o acceptor ./acceptor/main.go
go build -o initiator ./initiator/main.go
```

## Understanding the Code

### Application Callbacks

Both examples implement the `engine.Application` interface to handle FIX callbacks:

```go
type ExampleApp struct {
    engine.NoOpApplication
}

// Called when a session is created
func (a *ExampleApp) OnCreate(sessionID string) { ... }

// Called when Logon message is processed
func (a *ExampleApp) OnLogon(sessionID string) { ... }

// Called when Logout message is processed
func (a *ExampleApp) OnLogout(sessionID string) { ... }

// Called for application-level messages (e.g., NewOrder)
func (a *ExampleApp) FromApp(msg *fixmsg.FixMessage, sessionID string) error { ... }

// Called for any received message
func (a *ExampleApp) OnMessage(msg *fixmsg.FixMessage, sessionID string) { ... }
```

### Message Handling

Messages are represented as `*fixmsg.FixMessage`. Access fields by FIX tag number (integer):

```go
// Tag 35 is MsgType
msgType, _ := msg.Get(35)

// Tag 49 is SenderCompID
senderID, _ := msg.Get(49)

// Tag 108 is HeartBtInt (heartbeat interval)
heartbeatInt, _ := msg.Get(108)
```

### Engine Lifecycle

**Initiator:**
```go
// Create network initiator
init := network.NewInitiator("127.0.0.1:5001")

// Create engine
fe := engine.NewFixEngine(init)

// Register application
fe.SetApplication(app)

// Setup state machine and message store
fe.SetupComponents(state.NewStateMachine(), store.NewSQLiteStore())

// Connect
fe.Connect()

// Stop/shutdown
fe.Close()
```

**Acceptor:**
```go
// Create multi-session acceptor
m := engine.NewMultiSessionEngine("0.0.0.0:5001")

// Register application (called for each session)
m.SetApplication(app)

// Start accepting connections
m.Start()

// Stop accepting
m.Stop()
```

## Session IDs

Session IDs are formatted as: `"{SENDER}-{TARGET}-{HOST}:{PORT}"`

Example: `SENDER-TARGET-127.0.0.1:5001`

## Error Handling

Both examples demonstrate error handling:

- Failed initial connection (Initiator will retry with backoff)
- Configuration loading failures (gracefully continues with defaults)
- Signal handling for graceful shutdown (Ctrl+C)

## Sending Messages

To send a message, call `engine.SendMessage()`:

```go
msg := fixmsg.NewFixMessage()
msg.Set(35, "D")  // MsgType = NewOrder
msg.Set(55, "AAPL")  // Symbol
msg.Set(38, 100)  // OrderQty
// ... set other fields

if err := fe.SendMessage(msg); err != nil {
    log.Printf("Failed to send message: %v", err)
}
```

## Advanced Topics

### Reconnection Strategy

Configure automatic reconnection with exponential backoff:

```go
fe.SetReconnectParams(
    2*time.Second,    // initial backoff
    30*time.Second,   // max backoff
    true,             // randomize
)
```

### Message Store

FIX engines require a persistent message store to maintain session state and message history for resynchronization. GoFixMsg provides SQLite-based storage.

#### SQLite Message Store

**Synchronous SQLite Store (Recommended):**

```go
import (
    "github.com/wxcuop/gofixmsg/engine"
    "github.com/wxcuop/gofixmsg/state"
    "github.com/wxcuop/gofixmsg/store"
)

// Create engine
fe := engine.NewFixEngine(network.NewInitiator("127.0.0.1:5001"))
fe.SetApplication(app)

// Create state machine
sm := state.NewStateMachine()

// Create SQLite message store
msgStore := store.NewSQLiteStore()

// Initialize store with database file path
if err := msgStore.Init("fix_messages.db"); err != nil {
    log.Fatalf("Failed to initialize store: %v", err)
}

// Wire components
fe.SetupComponents(sm, msgStore)
```

**How it Works:**
1. **Database File**: Creates/uses `fix_messages.db` at the specified path
2. **Schema**: Automatically creates tables for:
   - `messages` - Stores parsed FIX messages by (BeginString, SenderCompID, TargetCompID, MsgSeqNum)
   - `sessions` - Stores session state including outbound and inbound sequence numbers
3. **Concurrency**: Uses SQLite's WAL (Write-Ahead Logging) mode for better concurrent read performance
4. **Pragmas**: Configured with `journal_mode=WAL` and `synchronous=NORMAL`

**Example Database Contents:**
```sql
-- Messages table
SELECT * FROM messages 
WHERE sendercompid='CLIENT' AND targetcompid='SERVER';
-- Output:
-- | BeginString | SenderCompID | TargetCompID | MsgSeqNum | MsgType | Body | Created |
-- | FIX.4.4      | CLIENT       | SERVER       | 1         | A       | ... | 1709297...

-- Sessions table
SELECT * FROM sessions;
-- Output:
-- | SessionID                   | Seq | In_Seq |
-- | CLIENT-SERVER-127.0.0.1:5001| 42  | 50    |
```

**Message Store Operations:**

```go
// Saving a message
msg := fixmsg.NewFixMessage()
msg.Set(fixmsg.TagBeginString, "FIX.4.4")
msg.Set(fixmsg.TagMsgType, "D")
// ... set other fields

storeMsg := &store.Message{
    BeginString:  msg.MustGet(fixmsg.TagBeginString),
    SenderCompID: msg.MustGet(fixmsg.TagSenderCompID),
    TargetCompID: msg.MustGet(fixmsg.TagTargetCompID),
    MsgSeqNum:    42,
    MsgType:      msg.MustGet(fixmsg.TagMsgType),
    Body:         []byte("...serialized message..."),
    Created:      time.Now(),
}

if err := msgStore.SaveMessage(storeMsg); err != nil {
    log.Printf("Failed to save message: %v", err)
}

// Retrieving a message
retrieved, err := msgStore.GetMessage("FIX.4.4", "CLIENT", "SERVER", 42)
if err != nil {
    log.Printf("Failed to get message: %v", err)
}

// Saving session state (called automatically during logon)
if err := msgStore.SaveSessionSeq("CLIENT-SERVER-127.0.0.1:5001", 43, 51); err != nil {
    log.Printf("Failed to save session seq: %v", err)
}

// Retrieving session state
outSeq, inSeq, err := msgStore.GetSessionSeq("CLIENT-SERVER-127.0.0.1:5001")
```

**Database File Location:**

```go
// Current directory
msgStore.Init("fix_messages.db")

// Specific directory
msgStore.Init("/tmp/fix_data/messages.db")

// Using absolute path
dir := os.ExpandEnv("$HOME/.fix")
msgStore.Init(filepath.Join(dir, "store.db"))
```

**Performance Characteristics:**
- **Writes**: ~1-2ms per message (dependent on disk I/O)
- **Reads**: ~0.5-1ms per message
- **Concurrent access**: Safe for multi-session usage via SQLite WAL
- **Disk usage**: ~300 bytes per stored message (varies with message size)

**Cleanup and Retention:**

```go
// Manual cleanup of old messages (application-specific)
// For example, remove messages older than 30 days:
query := `DELETE FROM messages 
          WHERE created < datetime('now', '-30 days')`

// To do this, connect to the database directly:
import "database/sql"
_ "modernc.org/sqlite"

db, _ := sql.Open("sqlite", "fix_messages.db")
defer db.Close()
db.Exec(query)
```

**Configuration (Optional):**

Store initialization is typically called once during engine setup:

```go
// Full initialization example
msgStore := store.NewSQLiteStore()

// Default init (creates in current directory)
if err := msgStore.Init("fix_sessions.db"); err != nil {
    log.Fatal(err)
}

// Now ready to use with engine
fe.SetupComponents(state.NewStateMachine(), msgStore)
```

**Important Notes:**
- SQLite is single-threaded, but `modernc.org/sqlite` handles serialization internally
- Messages persist across application restarts
- Enables automatic gap fills during reconnection
- Sequence numbers are persisted and survived restarts
- Do not access the same database from multiple engine instances simultaneously (use separate DB files or implement locking)

### TLS/SSL

Configure TLS in `config.ini`:

```ini
[Session]
host = 127.0.0.1
port = 5001
ssl_cert_file = /path/to/cert.pem
ssl_key_file = /path/to/key.pem
ssl_ca_file = /path/to/ca.pem      # For certificate verification
```

## Troubleshooting

### Connection Refused
- Ensure acceptor is running and listening on the specified address/port
- Check firewall rules

### Logon Failures
- Verify `sender_comp_id` and `target_comp_id` match between both sides
- Check heartbeat interval settings

### Message Ordering
- Ensure sequence numbers are enabled (automatic)
- Check message store is properly initialized

## Message Parsing and Creation Tools

GoFixMsg includes command-line tools for working with FIX messages without writing code.

### Parse Tool (`cmd/parse/main.go`)

Parse FIX messages from the command-line or from files:

```bash
# Basic parse with tag numbers
go run ../cmd/parse/main.go -input "8=FIX.4.4|9=50|35=A|49=SENDER|56=TARGET|108=30|10=123"

# Show tag names alongside numbers
go run ../cmd/parse/main.go -input "..." -tags

# Pretty-print with table layout
go run ../cmd/parse/main.go -input "..." -pretty

# Parse from file
echo "8=FIX.4.4|9=50|35=A|49=SENDER|56=TARGET|108=30|10=123" > message.fix
go run ../cmd/parse/main.go -file message.fix -pretty

# Combine options
go run ../cmd/parse/main.go -file message.fix -pretty -tags
```

**Usage:**
```
-input string    FIX message (use | for SOH character)
-file string     File containing FIX message
-pretty          Pretty-print output (table format)
-tags            Show tag names in addition to numbers
```

**Notes:**
- The tool uses `|` as a placeholder for SOH (0x01) in string input for readability
- When parsing files, use actual SOH characters (0x01) or raw binary data
- Output shows tag numbers, values, and optionally standard FIX tag names

### Create Tool (`cmd/create/main.go`)

Create FIX messages from the command-line:

```bash
# Create a Logon message (minimal)
go run ../cmd/create/main.go -type A -sender TRADER -target EXCHANGE

# Create a New Order Single with application fields
go run ../cmd/create/main.go -type D \
  -sender TRADER -target EXCHANGE \
  -fields "55=AAPL,54=1,40=2,38=1000,44=150.25"

# Show message details during creation
go run ../cmd/create/main.go -type D \
  -fields "55=MSFT,54=2,40=1" \
  -pretty

# Output in readable format (| for SOH)
go run ../cmd/create/main.go -type D \
  -fields "55=AAPL,54=1" \
  -ascii

# Save binary message to file
go run ../cmd/create/main.go -type D \
  -fields "55=AAPL,54=1,40=2,38=500" \
  -output order.fix

# Use raw binary output in pipeline
go run ../cmd/create/main.go -type 0 > heartbeat.fix
xxd heartbeat.fix
```

**Usage:**
```
-type string      Message type (A=Logon, D=NewOrder, 8=ExecutionReport, etc.)
-sender string    Sender CompID (default: SENDER)
-target string    Target CompID (default: TARGET)
-seq int          Message sequence number (default: 1)
-fields string    Additional fields as tag=value,tag=value...
-pretty           Show message details
-ascii            Output with | instead of SOH
-output string    Write to file instead of stdout
```

**Common Message Types:**
- `A` = Logon
- `D` = New Order Single
- `8` = Execution Report
- `9` = Order Cancel Request
- `0` = Heartbeat
- `5` = Logout

**Field Examples:**
```
# Symbol and quantity
-fields "55=AAPL,38=1000"

# Side (1=Buy, 2=Sell), order type (1=Market, 2=Limit)
-fields "54=1,40=2"

# Price
-fields "44=150.25"

# Multiple fields
-fields "55=AAPL,54=1,40=2,38=500,44=150.25,11=CLORD-001"
```

## Library Examples

Use GoFixMsg as a library to build FIX applications:

### Basic Message Creation and Parsing

```go
import "github.com/wxcuop/gofixmsg/fixmsg"

// Create a message
msg := fixmsg.NewFixMessage()
msg.Set(fixmsg.TagBeginString, "FIX.4.4")
msg.Set(fixmsg.TagMsgType, "D")
msg.Set(fixmsg.TagSenderCompID, "TRADER")
msg.Set(fixmsg.TagTargetCompID, "EXCHANGE")
msg.Set(fixmsg.TagClOrdID, "ORDER-001")
msg.Set(55, "AAPL")  // Symbol
msg.Set(54, "1")     // Buy
msg.Set(38, "1000")  // Quantity
msg.Set(44, "150.25") // Price

// Serialize to wire format
wire, _ := msg.ToWire()

// Parse from wire format
parsed := fixmsg.NewFixMessage()
parsed.LoadFix(wire)

// Access fields
msgType := parsed.Get(fixmsg.TagMsgType)
symbol := parsed.Get(55)
```

### Bulk Message Creation

```go
// Create from map
fields := map[int]string{
    fixmsg.TagBeginString: "FIX.4.4",
    fixmsg.TagMsgType: "D",
    fixmsg.TagSenderCompID: "TRADER",
    fixmsg.TagSymbol: "AAPL",
    fixmsg.TagSide: "1",
    fixmsg.TagOrderQty: "1000",
}
msg := fixmsg.NewFixMessageFromMap(fields)
```

### Field Access Patterns

```go
// By integer tag
value := msg.Get(55)  // Symbol

// By constant
value := msg.Get(fixmsg.TagSymbol)

// Check existence
if v, exists := msg.FixFragment[55]; exists {
    fmt.Printf("Symbol: %s\n", v)
}

// Iterate all fields
for tag, value := range msg.FixFragment {
    fmt.Printf("Tag %d: %s\n", tag, value)
}
```

### Running Library Examples

```bash
cd examples/parser
go run main.go
```

This runs practical examples demonstrating:
1. Creating a Logon message
2. Creating a New Order Single
3. Parsing received messages
4. Building complex messages
5. Working with various field access patterns

See [examples/parser/main.go](parser/main.go) for complete detailed code.

### Comprehensive Examples 

Run comprehensive examples covering all major FIX message operations:

```bash
cd examples/comprehensive
go run main.go
```

**Output:**
```
Loaded FIX spec from ../../FIX44.xml (version FIX.4.4)

1. Vanilla Tag/Value Parsing and Access
  ...
```

This demonstrates:
1. **Vanilla Tag/Value Parsing** - Parse raw FIX messages and access fields
2. **Tag Comparison Operations** - Exact matches, contains, regex matching, case-insensitive comparisons
3. **Tag Manipulation** - Set, update, and delete fields; SetOrDelete functionality
4. **Message Copying** - Efficient message copying via serialization roundtrip
5. **Repeating Groups** - Work with FIX repeating groups (NoMDEntries, etc.) with full spec support
6. **Field Paths** - Use FindAll() to locate tags at various depths

**Note:** This example automatically loads FIX44.xml for full repeating group support.

See [examples/comprehensive/main.go](comprehensive/main.go) for complete code.

### Advanced Spec and Serialization Examples

```bash
cd examples/specs
go run main.go
```

**Output:**
```
Loaded FIX spec from ../../FIX44.xml (version FIX.4.4)

1. Standard Codec Usage
Using FIX spec FIX.4.4 for parsing
  ...
```

This demonstrates:
1. **Standard Codec Usage** - Parsing with full FIX spec support for repeating groups
2. **Serialization/Deserialization** - Message roundtrip from creation to wire format and back
3. **Field Type Handling** - Understanding how fields are stored and compared in Go
4. **Building Complex Messages** - Creating multi-field business messages from scratch

Topics include:
- Wire format representation
- Complete roundtrip parsing (serialize → parse)
- Field type handling and best practices
- Building production-ready business messages
- Full repeating group support via FIX spec

**Note:** This example automatically loads FIX44.xml for complete spec support.

See [examples/specs/main.go](specs/main.go) for complete code.

Since gofixmsg is both a **parser library** and a **session engine**, it can be used standalone to parse/create FIX messages without session management. The CLI tools demonstrate this standalone functionality, while the session examples (initiator/acceptor) show integrated usage with session management.

## References

- [GoFixMsg Architecture](../doc/plan.md)
- [Configuration Guide](../doc/migration.md#configuration-mapping)
- [FIX Protocol Specification](../doc/spec/)
