# Go Rewrite Task Tracking

## Completed Phases (50/43 todos done - exceeds estimates)

### Phase 1: FIX Message Foundation
- [x] `go.mod` setup with dependencies
- [x] `fixmsg/fixmessage.go` - FixMessage struct with Codec interface
- [x] `fixmsg/fragment.go` - FixFragment map type
- [x] `fixmsg/repeatinggroup.go` - RepeatingGroup support
- [x] `fixmsg/codec/codec.go` - Parse/Serialise with SOH delimiters
- [x] `fixmsg/spec/spec.go` - XML spec loader (FixSpec)
- [x] `fixmsg/tags.go` - Tag constants and mappings

### Phase 2: Configuration & Security
- [x] `config/configmanager.go` - INI reader with decryption
- [x] `crypt/simplecrypt.go` - PBKDF2 + AES-GCM

### Phase 3-5: Infrastructure
- [x] `idgen/clordid.go` - ClOrdID generators
- [x] `store/sqlite.go` - SQLite message store
- [x] `state/statemachine.go` - FIX state machine

### Phase 6-8: Network & Handlers
- [x] `network/{initiator,acceptor}.go` - TCP wrappers with Conn wrapper
- [x] `engine/processor.go` - Message handler dispatch
- [x] `heartbeat/heartbeat.go` - Heartbeat scheduler

### Phase 9-11: Engine Assembly & Application Interface
- [x] `engine/engine.go` - FixEngine core with reconnect/backoff
- [x] `engine/application.go` - Callback interface (OnCreate, OnLogon, OnLogout, etc.)
- [x] State machine events and transition logging

### Phase 12-18: Session & Admin Handling
- [x] Session framing, run loops, and sequence management
- [x] Admin handlers (Logon, Logout, ResendRequest, SequenceReset, TestRequest)
- [x] HeartbeatMonitor with configuration
- [x] State machine enhancements and logging

### Phase 19-20: Network Abstractions & TLS/Certs (COMPLETED)
- [x] Conn wrapper with Send/SetReadDeadline/SetWriteDeadline/Flush methods
- [x] Per-client goroutine in Acceptor with sync.WaitGroup for clean shutdown
- [x] Buffer size tuning (8192 bytes matching Python implementation)
- [x] network/tls.go LoadTLSConfig(certFile, keyFile, caFile)
- [x] TLS loading integration in SetupComponents (checks ssl_* config keys)
- [x] Comprehensive unit tests for Conn wrapper, TLS loading, per-client handling
- [x] Initiator and Acceptor updated to use Conn wrapper
- [x] *Conn implements net.Conn interface for backward compatibility

## Pending Phases

### Phase 21: ResendRequest/GapFill Hardening
- [ ] Edge case tests for ResetSeqNumFlag
- [ ] Incoming seq persistence scenarios
- [ ] ResetSeqNumFlag interaction with GapFill

### Phase 22+: Future Enhancements
- [ ] Robust framing with BodyLength (tag 9)
- [ ] Sequential processing guarantees
- [ ] FIX Dictionary validation hardening
- [ ] Acceptor multi-session support
- [ ] Package reorganization for maintainability

## Test Coverage
- [x] All 55+ existing tests passing
- [ ] New tests for `BodyLength` framing
- [ ] New tests for sequential processing
- [ ] New tests for hardened `ResendRequest`
