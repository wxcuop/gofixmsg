# Go Rewrite Plan

## Goal
Incrementally rewrite the Python pyfixmsg FIX engine into Go while preserving behavior and maintaining test coverage at each step. The Go version runs from `gorewrite/`, with unit and integration tests validating each phase.

## Approach

**Incremental phases** - each phase adds a complete, testable component:
1. Core message handling (codec, fragment, spec)
2. Config & crypto (INI reader, encryption)
3. Infrastructure (ID gen, persistence, state machine)
4. Network (Initiator/Acceptor)
5. Message dispatch (handlers, processor)
6. Engine assembly (FixEngine, session, admin handlers)
7. Heartbeat & monitoring (TestRequest escalation, session liveness)
8. Session lifecycle (OnClose callbacks, robust shutdown)
9. **Reconnect/Backoff (NEW - COMPLETE)** - exponential backoff with jitter, configurable policy
10. Future phases (app callbacks, TLS, ResendRequest edge cases)

**Constraints**:
- Minimize changes to existing Python code
- Preserve message store schema (SQLite)
- Run tests from gorewrite/ directory with `go test ./...`
- Keep all tests green as features are added
- Prefer queued Session.Send path for outbound writes
- Thread-safe components using sync primitives

**Testing Strategy**:
- Unit tests for each package
- Integration tests for end-to-end flows (logon, heartbeat, TestRequest escalation, reconnect)
- Both initiator and acceptor modes covered
- Partial TCP frame tolerance tested
- Config loading tested

## Current Status: Phase 16+ Complete (Reconnect/Backoff)

### What's Working (37 todos done)

**FIX Core**:
- Message parsing, serialization with SOH delimiters
- Repeating group support
- XML spec loading
- Tag constants and header/trailer mappings

**Configuration & Security**:
- INI-based config with ENC: encryption support
- PBKDF2 + AES-GCM cryptography

**Persistence**:
- SQLite message store
- Session sequence number persistence (survives restarts)

**Network**:
- TCP Initiator (NewInitiator, Connect, WithTLS)
- TCP Acceptor (Listen, Accept)

**Message Handling**:
- Processor for registering/dispatching handlers
- Admin handlers: Logon (with ResetSeqNumFlag), Logout, ResendRequest (with GapFill), SequenceReset, TestRequest

**Engine**:
- FixEngine assembly of components
- Session with ReadLoop (handles partial TCP frames), WriteLoop (queued writes), Send API
- SeqManager with persistence
- HeartbeatMonitor (TestRequest on timeout, closes session on 2.5× heartbeat interval)
- Engine-level heartbeat sender using configured CompIDs
- SendMessage centralizes header stamping (CompIDs, MsgSeqNum, SendingTime)

**Reconnect/Backoff (Phase 16 - NEW)**:
- Exponential backoff with jitter (1s→2s→4s→...→max 30s)
- FixEngine.startReconnectLoop() - thread-safe reconnect using context.Context
- Session.OnClose triggers reconnect when enableReconnect=true
- SetReconnectParams(initial, max, enable) API for tuning
- Reconnect disabled by default (test-safe)
- Integration test verifying reconnect logic

### Integration Tests

| Test | Path | Coverage |
|------|------|----------|
| Logon+Heartbeat | `integration/logon_heartbeat_test.go` | Initiator↔Acceptor TCP exchange, heartbeat intervals |
| TestRequest Escalation | `integration/testrequest_escalation_test.go` | Missed heartbeat detection, TestRequest→Heartbeat, session close on timeout |
| Reconnect Backoff | `integration/reconnect_backoff_test.go` | Reconnect params, backoff loop (NEW) |
| Engine+Store | `integration/integration_test.go` | Store persistence, FixEngine lifecycle |

### What's Pending (6 todos)

1. **Application Interface** (Phase 17)
   - Application interface with callbacks (OnCreate, OnLogon, OnLogout, ToAdmin, FromAdmin, ToApp, FromApp, OnReject)
   - Wire callbacks into handlers
   - Benefits: allows app-level message filtering and lifecycle hooks

2. **State Machine Events** (Phase 18)
   - Add missing events: client_accepted, initiate_reconnect, reconnect_failed_max_retries
   - slog logging on transitions
   - Benefits: better visibility into state machine behavior

3. **Network Abstractions** (Phase 19)
   - Conn wrapper with Send/SetReadDeadline
   - Per-client goroutine in Acceptor
   - Buffer size tuning
   - Benefits: cleaner network layer, matches Python buffering

4. **TLS Config from File** (Phase 20)
   - network/tls.go LoadTLSConfig()
   - Called by SetupComponents if config has ssl_* keys
   - Benefits: secure connections from config

5. **ResendRequest/GapFill Hardening** (Phase 21)
   - Edge case tests for ResetSeqNumFlag interactions
   - Incoming seq persistence
   - Benefits: matches Python edge-case behavior, fewer surprises in production

## File Structure

```
gorewrite/
├── go.mod                      # Go module definition
├── go.sum                      # Dependency lock file
├── doc/                        # Documentation
│   ├── plan.md                 # This file
│   ├── task.md                 # Task tracking
│   └── spec/
│       ├── spec.md             # Architecture overview
│       ├── code-style.md       # Go coding conventions
│       └── solid.md            # SOLID principles guide
├── fixmsg/                     # FIX message handling
│   ├── fixmessage.go
│   ├── fragment.go
│   ├── repeatinggroup.go
│   ├── tags.go
│   ├── codec/
│   │   └── codec.go
│   └── spec/
│       └── spec.go
├── config/                     # Configuration loading
│   └── configmanager.go
├── crypt/                      # Encryption/decryption
│   └── simplecrypt.go
├── idgen/                      # ID generation
│   └── idgen.go
├── store/                      # Persistence (SQLite)
│   └── sqlite.go
├── state/                      # State machine
│   └── statemachine.go
├── network/                    # TCP networking
│   ├── initiator.go
│   └── acceptor.go
├── handler/                    # Message handlers
│   └── processor.go
├── heartbeat/                  # Heartbeat scheduling
│   └── heartbeat.go
├── engine/                     # FIX engine core
│   ├── engine.go               # FixEngine, Connect, SetupComponents
│   ├── session.go              # Session framing, read/write loops
│   ├── attach.go               # AttachSession, DetachSession
│   ├── handlers.go             # Admin handlers (Logon, Logout, etc.)
│   ├── seqnum.go               # SeqManager with persistence
│   ├── heartbeat_monitor.go    # HeartbeatMonitor
│   ├── processor.go            # Processor (handler dispatch)
│   └── *_test.go               # Unit tests
└── integration/                # Integration tests
    ├── logon_heartbeat_test.go
    ├── testrequest_escalation_test.go
    ├── reconnect_backoff_test.go
    └── integration_test.go
```

## Running Tests

```bash
cd gorewrite
go test ./...          # All tests
go test ./engine       # Engine tests only
go test ./integration  # Integration tests only
go test -v -run TestReconnectBackoff  # Specific test
```

## Key Design Principles

1. **Minimal Changes** - Only modify Python code when absolutely necessary
2. **Test-Driven** - Each phase includes unit and integration tests
3. **Thread-Safe** - All shared state protected by sync primitives
4. **Clean Layering** - Dependencies flow upward (fixmsg → engine → app)
5. **Graceful Shutdown** - Sessions, monitors, and reconnect loops shut down cleanly
6. **Configuration-Driven** - Tunable heartbeat intervals, CompIDs, backoff params from config

## Next Actions

1. Implement application callback interface (Phase 17)
2. Add state machine event logging (Phase 18)
3. Refactor network layer abstractions (Phase 19)
4. Add TLS cert loading from config (Phase 20)
5. Harden ResendRequest/GapFill edge cases (Phase 21)

## Success Criteria

✅ Phase 1-16: Functional FIX engine with persistent sequences, heartbeat monitoring, and reconnect/backoff
- [x] Logon/Logout flows working over TCP
- [x] Heartbeat exchange with TestRequest escalation
- [x] Message sequence persistence and recovery
- [x] Exponential backoff reconnect with jitter
- [x] All 42+ tests passing
- [x] Config-driven tuning of heartbeat, CompIDs, reconnect params

Future phases (17-21):
- [ ] Application callbacks for message filtering
- [ ] State machine event visibility
- [ ] TLS support from config
- [ ] Edge case ResendRequest/GapFill matching Python behavior
