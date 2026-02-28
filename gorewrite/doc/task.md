# Go Rewrite Task Tracking

## Completed Phases (37/43 todos done)

### Phase 1: FIX Message Foundation
- [x] `go.mod` setup with dependencies (testify, config loaders)
- [x] `fixmsg/fixmessage.go` - FixMessage struct with Codec interface
- [x] `fixmsg/fragment.go` - FixFragment map type for tag/value pairs
- [x] `fixmsg/repeatinggroup.go` - RepeatingGroup support
- [x] `fixmsg/codec/codec.go` - Parse/Serialise with SOH delimiters
- [x] `fixmsg/spec/spec.go` - XML spec loader for QuickFIX files
- [x] `fixmsg/tags.go` - Tag constants and header/trailer mappings
- [x] Unit tests for all core message handling

### Phase 2: Configuration & Security
- [x] `config/configmanager.go` - INI reader with ENC: decryption support
- [x] `crypt/simplecrypt.go` - PBKDF2 + AES-GCM encryption/decryption

### Phase 3-5: Infrastructure
- [x] `idgen/idgen.go` - ClOrdID and message ID generators
- [x] `store/sqlite.go` - SQLite message store with session persistence
- [x] `state/statemachine.go` - FIX state machine with transitions

### Phase 6-8: Network & Handlers
- [x] `network/{initiator,acceptor}.go` - TCP connection wrappers
- [x] `handler/processor.go` - Message handler registry and dispatch
- [x] `heartbeat/heartbeat.go` - Heartbeat scheduler and monitor

### Phase 9-11: Engine Assembly
- [x] `engine/engine.go` - FixEngine core component
- [x] `engine/attach.go` - Session attach/detach with monitor/heartbeat wiring
- [x] `integration/logon_heartbeat_test.go` - Integration test

### Phase 12: Session Framing & Run Loops
- [x] `engine/session.go` - ReadLoop (partial frame tolerance), WriteLoop (send queue), Start/Stop
- [x] `engine/seqnum.go` - SeqManager with persistence via store
- [x] Session.Send API for queued writes
- [x] Integration test for partial TCP reads and send queue

### Phase 13: Admin Handlers
- [x] Logon handler with ResetSeqNumFlag(141) support and seqnum gap checking
- [x] Logout handler with graceful disconnect
- [x] ResendRequest handler with GapFill for missing messages
- [x] SequenceReset handler with GapFillFlag(123) support
- [x] TestRequest handler sending Heartbeat replies with TestReqID(112)
- [x] Refactored handlers to use queued Session.Send

### Phase 14: Heartbeat & Configuration
- [x] HeartbeatMonitor - detects missed heartbeats, sends TestRequest, closes on timeout
- [x] Engine-level heartbeat sender using SendMessage
- [x] Read heartbeat_interval from config (default 30s)
- [x] Read sender_comp_id and target_comp_id from config (defaults "S"/"T")
- [x] Stamped headers: CompIDs, MsgSeqNum (via SeqManager), SendingTime
- [x] Integration tests for logon+heartbeat and TestRequest escalation

### Phase 15: Session Lifecycle & Infrastructure (Partial)
- [x] Session.OnClose callback for lifecycle hooks
- [x] Session.abort() and robust Stop semantics
- [x] Graceful session cleanup on connection loss
- [ ] Application interface + callbacks (pending)
- [ ] State machine missing events (pending)
- [ ] Network send/receive abstractions (pending)
- [ ] TLS cert loading from config (pending)

### Phase 16: Reconnect/Backoff Manager (NEW - COMPLETED)
- [x] Exponential backoff with jitter (initial 1s, max 30s, doubles each retry)
- [x] FixEngine.startReconnectLoop() - thread-safe reconnect using context
- [x] Session.OnClose triggers reconnect when enabled
- [x] SetReconnectParams(initial, max, enable) API for tuning
- [x] enableReconnect flag defaults to false (test-safe)
- [x] Integration test verifying reconnect logic

## Completed Phases (41/43 todos done)

### Phase 1-16: [Previous phases completed]
- [All completed as documented above]

### Phase 17: Application Interface & Callbacks (COMPLETED)
- [x] `engine/application.go` - Application interface (OnCreate, OnLogon, OnLogout, ToAdmin, FromAdmin, ToApp, FromApp, OnReject)
- [x] Wire callbacks into handlers - ToAdmin/FromAdmin for admin messages, ToApp for app sends
- [x] NoOpApplication default implementation
- [x] OnCreate callback wired in AttachSession()
- [x] Processor automatically wraps app message handlers to call FromApp
- [x] SendMessage calls ToApp for non-admin messages
- [x] Fixed admin message types list (A/5/0/1/2/3/4 only)
- [x] Integration test demonstrating all callbacks

## Pending Phases (2 todos remaining)

### Phase 18: State Machine Events
- [ ] Add missing events: client_accepted, initiate_reconnect, reconnect_failed_max_retries
- [ ] slog logging on every state transition
- [ ] Return errors on undefined state+event combos

### Phase 19: Network Abstractions
- [ ] Conn wrapper with Send/SetReadDeadline
- [ ] Per-client goroutine in Acceptor
- [ ] Buffer size tuning (8192 bytes matching Python)

### Phase 20: TLS/Certs from Config
- [ ] network/tls.go LoadTLSConfig(certFile, keyFile, caFile)
- [ ] Called by SetupComponents if config has ssl_* keys

### Phase 21: ResendRequest/GapFill Hardening
- [ ] Edge case tests for ResetSeqNumFlag
- [ ] Incoming seq persistence scenarios
- [ ] ResetSeqNumFlag interaction with GapFill

## Test Coverage

All phases have unit and integration tests:
- `go test ./... -v` from gorewrite/ runs all 50+ tests
- Phases 12-17 include integration tests with full TCP logon/heartbeat/reconnect/callback flows
- Phase 17 test demonstrates all Application interface callbacks
- All existing tests pass (green)

## Key Architecture Decisions

1. **Session Framing**: FIX frame boundaries detected by searching for "10=NNN<SOH>" checksum tag
2. **Send Queue**: All outbound writes go via Session.sendCh to avoid concurrent writes
3. **Header Stamping**: SendMessage centralizes tag 49/56 (CompIDs), 34 (MsgSeqNum), 52 (SendingTime)
4. **Sequence Persistence**: SeqManager persists via store.SaveSessionSeq, survives restarts
5. **Reconnect Policy**: Exponential backoff with jitter, configurable via SetReconnectParams
6. **Test Safety**: Reconnect disabled by default; must explicitly call SetReconnectParams(..., enable=true)
