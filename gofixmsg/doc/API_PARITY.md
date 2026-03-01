# Python → Go API Parity Documentation

This document provides a comprehensive mapping between the Python FIX engine API (`pyfixmsg_plus`) and the Go implementation (`gofixmsg`).

## Purpose

Users migrating from Python to Go can use this document to:
1. Understand equivalent types and classes
2. Map callback functions
3. Update configuration files
4. Adapt message handling code

## Core Architecture Mapping

| Python Class/Module | Go Type/Package | Notes |
|---------------------|-----------------|-------|
| `pyfixmsg.FixMessage` | `fixmsg.FixMessage` | Dictionary-like message object; tags are integers |
| `pyfixmsg_plus.fixengine.FixEngine` | `engine.FixEngine` | Main session coordinator (initiator) |
| `pyfixmsg_plus.fixengine.engine.MultiSessionEngine` | `engine.MultiSessionEngine` | Multi-session coordinator (acceptor) |
| `pyfixmsg_plus.application.Application` | `engine.Application` | Interface for callbacks |
| `pyfixmsg_plus.fixengine.state_machine.StateMachine` | `state.StateMachine` | Session state management |
| `pyfixmsg_plus.fixengine.message_store.DatabaseMessageStore` | `store.SQLiteStore` | Message persistence |

## Application Callbacks

### Python (Asyncio)

```python
from pyfixmsg_plus.application import Application

class MyApp(Application):
    async def onCreate(self, session_id):
        """Called when session is created"""
        pass
    
    async def onLogon(self, session_id):
        """Called when Logon (35=A) is processed"""
        pass
    
    async def onLogout(self, session_id):
        """Called when Logout (35=5) is processed"""
        pass
    
    async def toAdmin(self, msg, session_id):
        """Called before sending admin message (35=A/0/1/etc.)"""
        pass
    
    async def fromAdmin(self, msg, session_id):
        """Called after receiving admin message"""
        pass
    
    async def toApp(self, msg, session_id):
        """Called before sending app message"""
        pass
    
    async def fromApp(self, msg, session_id):
        """Called after receiving app message"""
        pass
    
    async def onMessage(self, msg, session_id):
        """Called for any received message"""
        pass
```

### Go Equivalent

```go
package main

import "github.com/wxcuop/gofixmsg/engine"

type MyApp struct {
    engine.NoOpApplication  // Embeds default no-op implementations
}

// Called when session is created
func (a *MyApp) OnCreate(sessionID string) {
    // ...
}

// Called when Logon (35=A) is processed
func (a *MyApp) OnLogon(sessionID string) {
    // ...
}

// Called when Logout (35=5) is processed
func (a *MyApp) OnLogout(sessionID string) {
    // ...
}

// Called before sending admin message
func (a *MyApp) ToAdmin(msg *fixmsg.FixMessage, sessionID string) error {
    // return error if validation fails
    return nil
}

// Called after receiving admin message
func (a *MyApp) FromAdmin(msg *fixmsg.FixMessage, sessionID string) error {
    // ...
    return nil
}

// Called before sending app message
func (a *MyApp) ToApp(msg *fixmsg.FixMessage, sessionID string) error {
    // ...
    return nil
}

// Called after receiving app message
func (a *MyApp) FromApp(msg *fixmsg.FixMessage, sessionID string) error {
    // ...
    return nil
}

// Called for any received message
func (a *MyApp) OnMessage(msg *fixmsg.FixMessage, sessionID string) {
    // ...
}
```

**Key Differences:**
- Python: Async (`async def`), raise exceptions
- Go: Synchronous, return `error` or `nil`
- Python: Single optional method; Go: Override specific methods
- Go: Use `engine.NoOpApplication` to skip unneeded callbacks

## Message Object

### Python

```python
# Create a message
msg = FixMessage()

# Set values (keys are integers - FIX tag numbers)
msg[35] = "D"           # MsgType = NewOrder
msg[49] = "SENDER"      # SenderCompID
msg[56] = "TARGET"      # TargetCompID
msg[55] = "AAPL"        # Symbol
msg[38] = 100           # OrderQty

# Get values
msg_type = msg.get(35)
quantity = msg[38]

# Iterate
for tag, value in msg.items():
    print(f"Tag {tag}: {value}")

# Delete
del msg[55]

# Check existence
if 55 in msg:
    print("Symbol found")
```

### Go Equivalent

```go
// Create a message
msg := fixmsg.NewFixMessage()

// Set values (tag numbers are int32)
msg.Set(35, "D")            // MsgType = NewOrder
msg.Set(49, "SENDER")       // SenderCompID
msg.Set(56, "TARGET")       // TargetCompID
msg.Set(55, "AAPL")         // Symbol
msg.Set(38, int64(100))     // OrderQty

// Get values
msgType, _ := msg.Get(35)
quantity, _ := msg.Get(38)

// Iterate
msg.Iterate(func(tag int32, value interface{}) {
    fmt.Printf("Tag %d: %v\n", tag, value)
})

// Delete
msg.Delete(55)

// Check existence
if _, ok := msg.Get(55); ok {
    fmt.Println("Symbol found")
}
```

**Key Differences:**
- Python: Dict-like syntax; Go: Method calls
- Python: Flexible types; Go: Explicit types (int32 for tags, specific types for values)
- Python: No error checking; Go: Returns `(value, ok)` tuple

## Engine Creation and Lifecycle

### Python (Initiator)

```python
from pyfixmsg_plus.fixengine import FixEngine
from pyfixmsg_plus.application import Application
import asyncio

class MyApp(Application):
    # ... callbacks ...
    pass

async def main():
    # Create engine
    engine = FixEngine(
        app=MyApp(),
        sender_comp_id="SENDER",
        target_comp_id="TARGET",
        heartbeat_interval=30,
    )
    
    # Start (connect)
    await engine.start()
    
    # Send message
    msg = FixMessage()
    msg[35] = "D"
    await engine.send_message(msg)
    
    # Wait
    try:
        await asyncio.sleep(3600)  # Keep alive
    except KeyboardInterrupt:
        pass
    
    # Shutdown
    await engine.stop()

asyncio.run(main())
```

### Go Equivalent (Initiator)

```go
package main

import (
    "github.com/wxcuop/gofixmsg/engine"
    "github.com/wxcuop/gofixmsg/fixmsg"
    "github.com/wxcuop/gofixmsg/network"
    "github.com/wxcuop/gofixmsg/state"
    "github.com/wxcuop/gofixmsg/store"
)

type MyApp struct {
    engine.NoOpApplication
}

func main() {
    // Create network initiator
    initiator := network.NewInitiator("127.0.0.1:5001")
    
    // Create engine
    fe := engine.NewFixEngine(initiator)
    fe.SetApplication(&MyApp{})
    
    // Setup components
    fe.SetupComponents(
        state.NewStateMachine(),
        store.NewSQLiteStore(),
    )
    
    // Connect (blocking, runs in goroutine)
    if err := fe.Connect(); err != nil {
        log.Fatalf("Connect failed: %v", err)
    }
    
    // Send message (non-blocking)
    msg := fixmsg.NewFixMessage()
    msg.Set(35, "D")
    if err := fe.SendMessage(msg); err != nil {
        log.Printf("SendMessage failed: %v", err)
    }
    
    // Wait for termination
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    
    // Shutdown
    fe.Close()
}
```

**Key Differences:**
- Python: `await` keyword; Go: Goroutines run automatically
- Python: Linear async flow; Go: Concurrent goroutine loops
- Python: Send is async; Go: Send is sync (queued internally)
- Python: Blocking sleep; Go: Channel receive

## Configuration Mapping

### Python (configparser)

```python
from pyfixmsg_plus.fixengine.configmanager import ConfigManager

mgr = ConfigManager()
mgr.load("config.ini")

sender = mgr.get("Session", "sender_comp_id")
target = mgr.get("Session", "target_comp_id")
interval = mgr.get("Session", "heartbeat_interval")
```

### Go Equivalent

```go
package main

import "github.com/wxcuop/gofixmsg/config"

mgr := config.GetManager()
err := mgr.Load("config.ini")
if err != nil {
    log.Fatalf("Load config failed: %v", err)
}

sender := mgr.Get("Session", "sender_comp_id")
target := mgr.Get("Session", "target_comp_id")
interval := mgr.Get("Session", "heartbeat_interval")
```

### Configuration File Format (Identical)

Both Python and Go use the same `.ini` format:

```ini
[Session]
sender_comp_id = SENDER
target_comp_id = TARGET
heartbeat_interval = 30
host = 127.0.0.1
port = 5001
bind_address = 0.0.0.0
```

## Error Handling

### Python

```python
try:
    await engine.start()
except ConnectionRefusedError as e:
    print(f"Connection failed: {e}")
except Exception as e:
    print(f"Error: {e}")
```

### Go

```go
if err := fe.Connect(); err != nil {
    switch err {
    case network.ErrConnectionRefused:
        fmt.Printf("Connection refused: %v\n", err)
    default:
        fmt.Printf("Error: %v\n", err)
    }
}
```

## State Management

### Python State Machine

```python
# States are strings
state = "LoggedOn"
# Subscribe to state changes
engine.state_machine.subscribe(callback)
```

### Go State Machine

```go
// States are constants (int)
state := state.StateLoggedOn

// Subscribe to state changes
sm.Subscribe(func(newState int) {
    // Handle state change
})
```

## Session ID Format

Both implementations use the same session ID format:

```
"{SENDER}-{TARGET}-{HOST}:{PORT}"
```

Examples:
- `SENDER-TARGET-127.0.0.1:5001`
- `CLIENT1-SERVER1-192.168.1.100:5002`

## Multi-Session (Acceptor)

### Python

```python
from pyfixmsg_plus.fixengine import FixEngine  # Same FixEngine

class MyApp(Application):
    # App methods called for each session
    pass

async def main():
    acceptor = FixEngine(
        mode="acceptor",  # Set acceptor mode
        bind_address="0.0.0.0",
        bind_port=5001,
        app=MyApp(),
    )
    await acceptor.start()
    # Sessions created automatically on each connection
```

### Go

```go
import "github.com/wxcuop/gofixmsg/engine"

type MyApp struct {
    engine.NoOpApplication
}

func main() {
    // Different type for acceptor
    m := engine.NewMultiSessionEngine("0.0.0.0:5001")
    m.SetApplication(&MyApp{})
    
    if err := m.Start(); err != nil {
        log.Fatalf("Start failed: %v", err)
    }
    
    // Sessions created automatically on each connection
    
    m.Stop()
}
```

## TLS/SSL Configuration

Both use the same configuration file keys:

```ini
[Session]
ssl_cert_file = /path/to/cert.pem
ssl_key_file = /path/to/key.pem
ssl_ca_file = /path/to/ca.pem
```

**Python:**
```python
engine.load_tls(cert_file, key_file, ca_file)
```

**Go:**
```go
tlsConfig, err := network.LoadTLSConfig(certFile, keyFile, caFile)
// Use tlsConfig when creating initiator/acceptor
```

## Repeating Groups

### Python

```python
from pyfixmsg import FixSpec

spec = FixSpec("FIX44.xml")
msg = FixMessage(spec=spec)

# Add repeating group
msg.add(384, 0)  # NoExecInst
for i in range(3):
    msg[384, i, 527] = f"Exec{i}"  # ExecInst
```

### Go

```go
import "github.com/wxcuop/gofixmsg/fixmsg"

msg := fixmsg.NewFixMessage()

// Add repeating group
rg := fixmsg.NewRepeatingGroup(384)  // NoExecInst
for i := 0; i < 3; i++ {
    rg.Add(fixmsg.NewGroup())
    rg.Get(i).Set(527, fmt.Sprintf("Exec%d", i))  // ExecInst
}
msg.SetRG(384, rg)
```

## Common Migration Tasks

### Task: Create and Send NewOrder

**Python:**
```python
msg = FixMessage()
msg[35] = "D"        # MsgType
msg[49] = "SENDER"   # SenderCompID
msg[56] = "TARGET"   # TargetCompID
msg[55] = "AAPL"     # Symbol
msg[38] = 100        # OrderQty
msg[40] = "2"        # OrdType (limit)
msg[44] = 150.5      # Price
await engine.send_message(msg)
```

**Go:**
```go
msg := fixmsg.NewFixMessage()
msg.Set(35, "D")        // MsgType
msg.Set(49, "SENDER")   // SenderCompID
msg.Set(56, "TARGET")   // TargetCompID
msg.Set(55, "AAPL")     // Symbol
msg.Set(38, int64(100)) // OrderQty
msg.Set(40, "2")        // OrdType (limit)
msg.Set(44, 150.5)      // Price
fe.SendMessage(msg)
```

### Task: Handle ExecutionReport

**Python:**
```python
async def fromApp(self, msg, session_id):
    msg_type = msg.get(35)
    if msg_type == "8":  # ExecutionReport
        clord_id = msg.get(11)
        exec_id = msg.get(17)
        print(f"Exec: {clord_id} -> {exec_id}")
```

**Go:**
```go
func (a *MyApp) FromApp(msg *fixmsg.FixMessage, sessionID string) error {
    msgType, _ := msg.Get(35)
    if msgType == "8" {  // ExecutionReport
        clordID, _ := msg.Get(11)
        execID, _ := msg.Get(17)
        fmt.Printf("Exec: %v -> %v\n", clordID, execID)
    }
    return nil
}
```

### Task: Configure Reconnection

**Python:**
```python
engine.heartbeat.reconnect_params = (2, 30, True)  # (min, max, randomize)
```

**Go:**
```go
fe.SetReconnectParams(
    2*time.Second,    // initial backoff
    30*time.Second,   // max backoff
    true,             // randomize
)
```

## Type Conversions

When migrating code, note these type conversions:

| Python | Go | Notes |
|--------|-----|-------|
| `int` or `float` | `int32`, `int64`, `float32`, `float64` | Be explicit about size |
| `str` | `string` | FIX values are string-based |
| `bool` | `bool` | Same semantics |
| `dict` | `map[int32]interface{}` | Simplified for FIX messages |
| `None` / not present | `nil` | Check `ok` in tuple return |
| `Exception` | `error` | Check return values |

## Next Steps

1. **Review Examples**: See runnable code in `gofixmsg/examples/`
2. **Run Tests**: Execute `go test ./...` to verify your environment
3. **Build Incrementally**: Migrate one component at a time
4. **Test Thoroughly**: Compare Python and Go behavior in parallel
5. **Validate Production**: Run canary deployments before full cutover

## References

- [GoFixMsg Examples](../examples/)
- [Python Migration Guide](./migration.md)
- [Sunset Recommendation](./sunset_recommendation.md)
- [FIX Protocol Spec](./spec/)

---

For questions about specific components, refer to the inline documentation in each Go package.
