# Python to Go Migration Guide

This guide provides an overview of the key differences between the Python implementation of `pyfixmsg_plus` and the Go rewrite (`gorewrite`).

## Architecture Mapping

| Component | Python Implementation | Go Implementation |
|-----------|-----------------------|-------------------|
| Message Parser | `pyfixmsg.FixMessage` | `fixmsg/` package |
| Session Engine | `pyfixmsg_plus.fixengine` | `engine/` package |
| Network Layer | `pyfixmsg_plus.fixengine.network` | `network/` package |
| State Machine | `pyfixmsg_plus.fixengine.state_machine` | `state/` package |
| Configuration | `configparser` (`.ini`) | `config/` package (using `ini.v1`) |
| Concurrency | `asyncio` | Goroutines and Channels |

## Configuration Mapping

The Go implementation maintains compatibility with the Python `.ini` configuration format.

### Key Sections

| Section | Key | Python | Go |
|---------|-----|--------|----|
| `[Session]` | `sender_comp_id` | `SenderCompID` | `FixEngine.SenderCompID` |
| `[Session]` | `target_comp_id` | `TargetCompID` | `FixEngine.TargetCompID` |
| `[Session]` | `heartbeat_interval` | `HeartbeatInterval` | `FixEngine.heartbeatInterval` |
| `[Session]` | `ssl_cert_file` | `ssl_cert_file` | `network.LoadTLSConfig` |
| `[Scheduler]` | `schedules` | JSON schedules | `scheduler.RuntimeScheduler` |

## API Examples

### Python (Asyncio)

```python
class MyApp(Application):
    def on_logon(self, session_id):
        print(f"Logon successful: {session_id}")

    def from_app(self, msg, session_id):
        print(f"Received app message: {msg}")

async def main():
    app = MyApp()
    engine = FixEngine(app=app)
    await engine.start()
```

### Go (Goroutines)

```go
type MyApp struct {
    engine.NoOpApplication
}

func (a *MyApp) OnLogon(sessionID string) {
    fmt.Printf("Logon successful: %s
", sessionID)
}

func (a *MyApp) FromApp(msg *fixmsg.FixMessage, sessionID string) error {
    fmt.Printf("Received app message: %v
", msg)
    return nil
}

func main() {
    app := &MyApp{}
    fe := engine.NewFixEngine(network.NewInitiator("localhost:5001"))
    fe.App = app
    fe.SetupComponents(state.NewStateMachine(), store.NewSQLiteStore())
    
    if err := fe.Connect(); err != nil {
        log.Fatal(err)
    }
    // Block until close
    select {}
}
```

## Key Differences

### Concurrency Model
- **Python**: Uses a single-threaded event loop (`asyncio`). Blocking operations can starve the session.
- **Go**: Uses lightweight goroutines. Each session's read/write loops run independently.

### Database Concurrency (SQLite)
SQLite is single-threaded, but the two implementations handle this differently:

**Python**:
- Uses `asyncio.Lock` to serialize all database operations
- Each `SaveMessage()`, `GetMessage()`, etc. call acquires the lock before accessing SQLite
- Ensures serialized access but requires explicit lock management

**Go**:
- Uses the `modernc.org/sqlite` driver, which internally serializes database operations
- Enables WAL mode (`PRAGMA journal_mode=WAL`) for better concurrent read performance
- Multiple goroutines can call `SaveMessage()`, `GetMessage()` simultaneously without explicit locking—the driver queues operations internally
- Result: Go applications achieve higher throughput with thousands of concurrent sessions reading/writing to the same database

**Key takeaway**: Go eliminates the need for application-level database locks; concurrency is handled transparently by the driver + WAL pragmas, allowing free goroutine concurrency at the application level while SQLite operations remain safely serialized underneath.

### Error Handling
- **Python**: Uses exceptions (`raise`, `try/except`).
- **Go**: Uses explicit error returns (`func() (val, error)`).

### Performance
- **Go**: Offers significantly lower latency and higher throughput due to compiled execution and efficient memory management.
- **Go**: Better multi-core utilization for managing thousands of concurrent sessions.

## Production Cutover Checklist

1. **Verify Environment**:
   - [ ] Go runtime installed (1.21+)
   - [ ] Build and run `go test ./...` in the `gorewrite` directory.
2. **Configuration**:
   - [ ] Migrate existing `.ini` config files.
   - [ ] Validate certificate paths for TLS.
3. **Database Migration**:
   - [ ] Note: The SQLite schema for message storage is compatible.
4. **Validation**:
   - [ ] Run the Go version in a staging environment alongside the Python version.
   - [ ] Compare logs and message sequence numbers.
5. **Cutover**:
   - [ ] Stop Python service.
   - [ ] Update load balancer/DNS to point to the Go service.
   - [ ] Monitor for sequence gaps or connectivity issues.

## Rollback Plan

1. Stop the Go service.
2. Restore the previous Python environment.
3. Restart the Python service.
4. Verify sequence numbers (they should be persisted in the same SQLite database).
