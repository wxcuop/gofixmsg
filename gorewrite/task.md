# Go Rewrite — Task Breakdown

Each task is marked with its phase, estimated complexity (S/M/L/XL), and blocking dependencies.

---

## Phase 1 — FIX Message Core

| ID | Task | Complexity | Depends On |
|----|------|-----------|------------|
| P1-01 | Initialise Go module (`go.mod`) under `gorewrite/` | S | — |
| P1-02 | Implement `FixFragment` (`map[int]any` with helper methods: `Get`, `Set`, `Contains`, `AllTags`) | M | P1-01 |
| P1-03 | Implement `RepeatingGroup` (`[]FixFragment`) with `FindAll` and `Anywhere` | M | P1-02 |
| P1-04 | Implement `FixMessage` embedding `FixFragment`, add `Length()`, `Checksum()`, `ToWire()`, `LoadFix()` | L | P1-03 |
| P1-05 | Implement `FixSpec` XML loader (parse QuickFIX XML, build repeating group map, `FixTag` structs) | L | P1-01 |
| P1-06 | Implement `Codec` — `Parse(buf []byte) (*FixMessage, error)` and `Serialise(msg *FixMessage) ([]byte, error)` with SOH delimiter and repeating group support | XL | P1-04, P1-05 |
| P1-07 | Port header/trailer tag ordering and checksum/length auto-calculation | M | P1-06 |
| P1-08 | Unit tests: round-trip parse→serialise, checksum verification, repeating groups, malformed input | L | P1-06, P1-07 |

---

## Phase 2 — Configuration & Crypto

| ID | Task | Complexity | Depends On |
|----|------|-----------|------------|
| P2-01 | Implement `SimpleCrypt` (PBKDF2-SHA256 key derivation, HMAC-verified stream cipher using `golang.org/x/crypto`) | M | P1-01 |
| P2-02 | Unit tests for `SimpleCrypt`: encrypt/decrypt round-trip, cross-verify against Python output for same inputs | M | P2-01 |
| P2-03 | Implement `ConfigManager` (INI reader via `gopkg.in/ini.v1`, singleton via `sync.Once`, `Get`/`Set`/`SaveConfig`) | M | P2-01 |
| P2-04 | Implement transparent `ENC:` value decryption in `ConfigManager.Get(section, key string, decrypt bool)` | S | P2-01, P2-03 |
| P2-05 | Unit tests for `ConfigManager`: reads same `config.ini`, decrypts `ENC:` values, falls back correctly | M | P2-03, P2-04 |

---

## Phase 3 — ID Generators

| ID | Task | Complexity | Depends On |
|----|------|-----------|------------|
| P3-01 | Define `ClOrdIDGenerator` interface (`Encode(n int) string`, `Decode(s string) int`) | S | P1-01 |
| P3-02 | Implement `NumericClOrdIdGenerator` (length, seed, endpoint modulo) | M | P3-01 |
| P3-03 | Implement `YMDClOrdIdGenerator` (YMD prefix + counter) | S | P3-01 |
| P3-04 | Implement `MonthClOrdIdGenerator` (day-of-month prefix, embeds Numeric) | S | P3-02 |
| P3-05 | Implement `NyseBranchSeqGenerator` (alpha branch + 4-digit seq, reserved codes logic) | L | P3-01 |
| P3-06 | Implement `OSESeqGenerator` and `CHIXBranchSeqGenerator` | S | P3-05 |
| P3-07 | Unit tests: encode/decode round-trips for all generators, boundary conditions, reserved code rejection | M | P3-02–P3-06 |

---

## Phase 4 — Message Store

| ID | Task | Complexity | Depends On |
|----|------|-----------|------------|
| P4-01 | Define `MessageStore` interface (`StoreMessage`, `GetMessage`, `GetNextIncomingSeqNum`, `GetNextOutgoingSeqNum`, `IncrementIncomingSeqNum`, `IncrementOutgoingSeqNum`, `ResetSeqNums`, `Close`) | S | P1-01 |
| P4-02 | Implement `SQLiteMessageStore` using `database/sql` + `mattn/go-sqlite3` with identical schema to Python | L | P4-01 |
| P4-03 | Implement `Initialize(beginstring, sendercompid, targetcompid string) error` — load persisted sequence numbers | M | P4-02 |
| P4-04 | Implement sequence number persistence on every increment (write-through) | M | P4-02 |
| P4-05 | Implement `GetMessagesForResend(from, to int) ([]*FixMessage, error)` for gap-fill support | M | P4-02 |
| P4-06 | Unit tests: store/retrieve round-trip, sequence number persistence across reopen, cross-compatibility test with Python-written `.db` file | L | P4-02–P4-05 |

---

## Phase 5 — Session State Machine

| ID | Task | Complexity | Depends On |
|----|------|-----------|------------|
| P5-01 | Define `State` interface (`OnEvent(event string, sm *StateMachine) State`, `Name() string`) | S | P1-01 |
| P5-02 | Implement all state structs: `Disconnected`, `Connecting`, `LogonInProgress`, `AwaitingLogon`, `Active`, `LogoutInProgress`, `Reconnecting` with full transition tables | M | P5-01 |
| P5-03 | Implement `StateMachine` with `sync.RWMutex`, `OnEvent`, `Subscribe(func(string))`, `CurrentState() string` | M | P5-01, P5-02 |
| P5-04 | Unit tests: every (`state`, `event`) → `nextState` combination, subscriber notification, concurrent event delivery | M | P5-02, P5-03 |

---

## Phase 6 — Network Layer

| ID | Task | Complexity | Depends On |
|----|------|-----------|------------|
| P6-01 | Define `NetworkConnection` interface (`Connect`, `Send([]byte) error`, `Receive(handler func([]byte)) error`, `Disconnect() error`) | S | P1-01 |
| P6-02 | Implement `Initiator` using `net.Dial` / `tls.Dial` with reconnect hook | M | P6-01 |
| P6-03 | Implement `Acceptor` using `net.Listen` / `tls.Listen`, goroutine-per-client with handler factory | M | P6-01 |
| P6-04 | Implement TLS context builder (server-side cert/key, client-side CA verification, `tls.Config`) | M | P6-02, P6-03 |
| P6-05 | Integration tests: loopback TCP connect, TLS handshake with self-signed cert, send/receive raw bytes | M | P6-02–P6-04 |

---

## Phase 7 — Message Handlers

| ID | Task | Complexity | Depends On |
|----|------|-----------|------------|
| P7-01 | Define `MessageHandler` interface (`Handle(ctx context.Context, msg *fixmsg.FixMessage) error`) | S | P1-04 |
| P7-02 | Implement middleware logging wrapper (equivalent of Python's `logging_decorator`) | S | P7-01 |
| P7-03 | Implement `MessageProcessor` — dispatch by tag-35 value using `map[string]MessageHandler` | S | P7-01 |
| P7-04 | Implement `LogonHandler` (CompID validation, sequence number check, ResetSeqNumFlag, acceptor logon response) | L | P7-01, P4-01, P5-01 |
| P7-05 | Implement `HeartbeatHandler` (update last-received time, respond to TestRequest) | S | P7-01 |
| P7-06 | Implement `TestRequestHandler` (send Heartbeat with TestReqID echo) | S | P7-01 |
| P7-07 | Implement `ResendRequestHandler` (replay messages from store, send gap-fill for admin messages) | L | P7-01, P4-05 |
| P7-08 | Implement `SequenceResetHandler` (GapFill and hard reset cases) | M | P7-01 |
| P7-09 | Implement `LogoutHandler` (echo logout if initiator, trigger state transition) | M | P7-01, P5-03 |
| P7-10 | Implement `RejectHandler`, `ExecutionReportHandler`, `NewOrderHandler`, `CancelOrderHandler`, `OrderCancelReplaceHandler`, `OrderCancelRejectHandler`, `NewOrderMultilegHandler`, `MultilegOrderCancelReplaceHandler` — delegate to `Application` interface | M | P7-01 |
| P7-11 | Unit tests: each handler with mock store, state machine, and application | L | P7-04–P7-10 |

---

## Phase 8 — Heartbeat & Scheduler

| ID | Task | Complexity | Depends On |
|----|------|-----------|------------|
| P8-01 | Implement `Heartbeat` struct with `Start(ctx context.Context)`, `Stop()`, `UpdateLastReceived()`, `UpdateLastSent()` using `time.Ticker` | M | P5-03 |
| P8-02 | Implement missed-heartbeat detection and TestRequest escalation logic | M | P8-01 |
| P8-03 | Implement `HeartbeatBuilder` (fluent builder, method chaining) | S | P8-01 |
| P8-04 | Implement `Scheduler` — parse JSON schedules from config, fire actions at wall-clock times using `time.AfterFunc` | M | P2-03 |
| P8-05 | Unit tests: heartbeat fires at correct interval, missed-heartbeat triggers TestRequest, scheduler actions fire at correct time (with time mocking) | M | P8-01–P8-04 |

---

## Phase 9 — FIX Engine Assembly

| ID | Task | Complexity | Depends On |
|----|------|-----------|------------|
| P9-01 | Define `Application` interface (`OnCreate`, `OnLogon`, `OnLogout`, `ToAdmin`, `FromAdmin`, `ToApp`, `FromApp`, `OnMessage`) | S | P1-04 |
| P9-02 | Implement `FixEngine` struct, constructor `NewFixEngine`, field initialisation from config | M | P2-03, P4-02, P5-03, P6-01, P7-03, P8-01, P9-01 |
| P9-03 | Implement `Connect()` — TCP connect, start receive loop (goroutine), trigger state event | M | P9-02, P6-02 |
| P9-04 | Implement `SendLogon()`, `SendLogout()`, `SendHeartbeat()`, `SendMessage(msg)` with header stamping (seqnum, sending time, comp IDs) | L | P9-02, P4-02 |
| P9-05 | Implement receive loop — read bytes, parse FIX messages, dispatch to `MessageProcessor` | L | P9-02, P7-03 |
| P9-06 | Implement reconnect loop with configurable back-off (`retry_interval`, `max_retries`) | M | P9-03, P5-03 |
| P9-07 | Implement acceptor mode — `StartAccepting()`, per-client engine instance via handler factory | M | P9-02, P6-03 |
| P9-08 | Implement sequence number gap detection and automatic ResendRequest | M | P9-05, P4-02 |
| P9-09 | Integration test: Go initiator ↔ Go acceptor full session (logon, heartbeat, logout) | L | P9-03–P9-07 |
| P9-10 | Integration test: Go initiator ↔ Python acceptor (or QuickFIX) | L | P9-09 |

---

## Phase 10 — CLI Tools

| ID | Task | Complexity | Depends On |
|----|------|-----------|------------|
| P10-01 | Implement `cmd/query/main.go` — `--config`, `--session`, `--seqnum`, `--clordid` flags, query SQLite store | M | P4-02, P2-03 |
| P10-02 | Verify output parity with Python `tools/query.py` against same `.db` file | S | P10-01 |

---

## Phase 11 — Integration & Python Sunset

| ID | Task | Complexity | Depends On |
|----|------|-----------|------------|
| P11-01 | Update CI workflow to build and test Go module | S | P9-09 |
| P11-02 | Update `copilot-instructions.md` to document Go module structure | S | P11-01 |
| P11-03 | Move Python source to `_python_archive/` | S | P11-01 |
| P11-04 | Remove Python-only CI steps | S | P11-03 |
