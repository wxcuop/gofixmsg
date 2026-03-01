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

### Phase 22-23: BodyLength Framing & Sequential Processing (COMPLETED)
- [x] Phase 22: Implement BodyLength (tag 9) based framing in Session.readLoop
- [x] Parse tag 9 value and use as frame delimiter instead of "10="
- [x] Handle partial BodyLength reads across TCP packets
- [x] Correctly skip "10=" appearing in message data fields
- [x] Phase 23: Remove per-message goroutines from Session.readLoop
- [x] Serialize message handling to guarantee sequential order
- [x] 13 unit tests for BodyLength framing (edge cases, frame splits, data fields)
- [x] 7 unit tests for sequential processing (concurrency verification, ordering)
- [x] Merged to master with commit 3dd5e1f (27/27 engine tests passing)

## Pending Phases - Organized for Parallel Work

### 🔵 PARALLEL GROUP 1: Hardening & Testing (Independent)

#### Phase 21: ResendRequest/GapFill Hardening
**Worktree:** `p21-resendrequest-hardening`
**Dependencies:** handler/logon.go, handler/resendrequest.go, engine/seqnum.go
**Estimated:** 2-3 hours
- [ ] Edge case tests for ResetSeqNumFlag(141)
  - Test with seqnum reset and gap detection
  - Verify backward compatibility
- [ ] Incoming seq persistence scenarios
  - Session recovery with stored sequences
  - Verify store.Get/Store operations
- [ ] ResetSeqNumFlag(141) interaction with GapFill(123)
  - Ensure correct message replay
  - Validate gap fill flag handling

#### Phase 24: FIX Dictionary Validation Hardening
**Worktree:** `p24-dictionary-validation`
**Dependencies:** engine/processor.go, fixmsg/spec.go (independent from Phase 21)
**Estimated:** 2-3 hours
- [ ] Wire FixSpec more comprehensively into Processor
  - Check all required fields per message type
  - Validate field presence early
- [ ] Validate more field types beyond basics
  - UTCTimestamp, Price, Qty, etc.
  - Data type coercion and validation
- [ ] Add field value range validation
  - Min/max bounds per field
  - Enum value validation

---

### 🟢 PARALLEL GROUP 2: Session Processing (Sequential within group) - COMPLETED ✅

#### Phase 22: Robust Framing with BodyLength (tag 9) - ✅ COMPLETED
**Worktree:** `p22-23-session-framing` (combined) - Merged to master
**Status:** DONE - Commit 3dd5e1f
**Completed:** 1.5-2 hours
- [x] Implement BodyLength (tag 9) based framing in Session.readLoop
  - Parse tag 9 value
  - Use as frame delimiter instead of "10="
  - Handle partial BodyLength reads
- [x] Add unit tests for partial reads
  - Frame split mid-tag
  - Frame split mid-value
  - BodyLength in data field
- [x] Handle "10=" in data fields correctly
  - Checksum (tag 10) detection after BodyLength
  - Escape sequence handling

#### Phase 23: Sequential Processing Guarantees - ✅ COMPLETED
**Depends On:** Phase 22 (BodyLength framing) - ✅ Satisfied
**Status:** DONE - Merged with Phase 22
**Completed:** 1.5-2 hours
- [x] Remove per-message goroutines in Session.readLoop
  - Serialize message handling
  - Remove goroutine spawning
- [x] Ensure HandleIncoming called sequentially
  - Single-threaded message processing
  - Maintain exact sequence order
- [x] Verify sequence integrity in integration tests
  - Multi-message sequences
  - Validate MsgSeqNum ordering

---

### 🟡 PARALLEL GROUP 3: Multi-Session Support (Independent)

#### Phase 25: Acceptor Multi-Session Support
**Worktree:** `p25-multi-session`
**Dependencies:** engine/engine.go, network/acceptor.go
**Estimated:** 2-3 hours
- [ ] Full FixEngine support for multiple concurrent sessions
  - Session map keyed by session ID
  - Per-session heartbeat monitors
  - Per-session state machines
- [ ] Standardized session ID format
  - Format: `BeginString:SenderCompID:TargetCompID`
  - Consistent session ID generation
  - Session lookup optimization
- [ ] Integration test for multi-session acceptor
  - 2+ concurrent initiators
  - Verify session isolation
  - Heartbeat and message routing per session

---

### 🟣 SEQUENTIAL GROUP 4: Refactoring (Last, after others merge)

#### Phase 26: Package Reorganization
**Worktree:** `p26-refactoring`
**Prerequisites:** All other phases should be merged first
**Estimated:** 1-2 hours
- [ ] Move engine/session.go to engine/session/
  - Reduce engine/ package size
  - Better organization
- [ ] Move engine/processor.go to engine/handler/
  - Group all handlers together
  - Cleaner package structure
- [ ] Clean up circular dependencies
  - Update imports
  - Verify build succeeds
  - Update tests

---

## Test Coverage Summary
- [x] All 55+ existing tests passing (Phase 1-20)
- [x] 13 new tests for `BodyLength` framing (Phase 22)
- [x] 7 new tests for sequential processing (Phase 23)
- [x] 27/27 engine tests passing after Phase 22-23 merge
- [ ] 3 new tests for ResendRequest edge cases (Phase 21)
- [ ] 3 new tests for FIX dictionary validation (Phase 24)
- [ ] 3 new tests for multi-session support (Phase 25)
- [ ] 3 new tests for package reorganization (Phase 26)

---

## Parallel Work Strategy

### ✅ Recommended Execution Order

**Parallel Group 1 & 2 & 3 (Can run simultaneously):**
1. Start `p21-resendrequest-hardening` (2-3 hrs)
2. Start `p24-dictionary-validation` (2-3 hrs)
3. Start `p22-23-session-framing` (3-4 hrs, internal sequence)
4. Start `p25-multi-session` (2-3 hrs)

**Merge when complete (in any order):**
- Merge p21 → master
- Merge p24 → master
- Merge p22-23 → master (keep order: 22, then 23)
- Merge p25 → master

**Sequential (after all above):**
5. Start `p26-refactoring` (1-2 hrs) on clean master
6. Merge p26 → master

**Total elapsed time with parallel work: ~4-5 hours vs 12-14 hours sequential**

---

## Phase Dependencies

```
Phase 21 ──┐
           ├─→ [Independent, can merge in any order]
Phase 24 ──┤
           │
Phase 22 ──→ Phase 23 ──┤
           │
Phase 25 ──┴─────────────→ Master branch clean

Then (after all merged):
Phase 26 ──→ Master branch with refactoring
```
