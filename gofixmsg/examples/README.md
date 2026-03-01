# GoFixMsg Examples

This directory contains runnable examples demonstrating how to build FIX applications using GoFixMsg.

## Overview

The examples show:
- **initiator/** - Connecting to a remote FIX peer as an Initiator
- **acceptor/** - Accepting connections from FIX peers as an Acceptor

## Prerequisites

- Go 1.21 or later
- A `config.ini` file (see below)

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

Choose a message store implementation:

```go
// SQLite (synchronous)
fe.SetupComponents(state.NewStateMachine(), store.NewSQLiteStore())
```

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

## Migration from Python

See [Migration Guide](../doc/migration.md) for mapping Python code to Go.

## References

- [GoFixMsg Architecture](../doc/plan.md)
- [Configuration Guide](../doc/migration.md#configuration-mapping)
- [Python Migration](../doc/migration.md)
- [FIX Protocol Specification](../doc/spec/)
