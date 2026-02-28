# Go Rewrite Task Tracking

## Completed Phases (45+ todos done)

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
- [x] `network/{initiator,acceptor}.go` - TCP wrappers
- [x] `engine/processor.go` - Message handler dispatch
- [x] `heartbeat/heartbeat.go` - Heartbeat scheduler

### Phase 9: Reconnect/Backoff
- [x] Exponential backoff with jitter
- [x] `FixEngine.startReconnectLoop()`
- [x] Integration tests for reconnect

### Phase 10: Application Interface
- [x] `engine/application.go` - Callback interface
- [x] Wire callbacks into admin and app handlers
- [x] `OnCreate`, `OnLogon`, `OnLogout`, `ToAdmin`, `FromAdmin`, `ToApp`, `FromApp`, `OnReject`

### Phase 11: State Machine & Logging
- [x] Transition logging with `slog`
- [x] Events: `ClientAccepted`, `InitiateReconnect`, `ReconnectFailedMax`

## Pending Phases (New Robustness Focus)

### Phase 12: Robust Framing
- [ ] Implement `BodyLength` (tag 9) based framing in `Session.readLoop`
- [ ] Add unit tests for partial reads and "10=" in data fields
- [ ] Remove naive `10=` splitting

### Phase 13: Sequential Processing
- [ ] Remove per-message goroutines in `Session.readLoop`
- [ ] Ensure `HandleIncoming` is called sequentially
- [ ] Verify sequence integrity in integration tests

### Phase 14: ResendRequest Hardening
- [ ] Add `PossDupFlag(43)=Y` to replayed messages
- [ ] Add `OrigSendingTime(122)` to replayed messages
- [ ] Ensure replayed messages use original `MsgSeqNum`
- [ ] Test edge cases for sequence gaps and `GapFill`

### Phase 15: FIX Dictionary Validation
- [x] Wire `FixSpec` into `Processor` for message validation
- [x] Check for mandatory fields (tag 8, 9, 35, 49, 56, 34, 52, 10)
- [x] Validate data types (int, float, UTCTimestamp)

### Phase 16: Acceptor Integration
- [ ] Full `FixEngine` support for multiple concurrent sessions (Acceptor mode)
- [ ] Standardized session ID format: `BeginString:SenderCompID:TargetCompID`
- [ ] Integration test for multi-session acceptor

### Phase 17: Package Reorganization
- [ ] Move `engine/session.go` to `engine/session/`
- [ ] Move `engine/processor.go` and handlers to `engine/handler/`
- [ ] Clean up circular dependencies

## Test Coverage
- [x] All 55+ existing tests passing
- [ ] New tests for `BodyLength` framing
- [ ] New tests for sequential processing
- [ ] New tests for hardened `ResendRequest`
