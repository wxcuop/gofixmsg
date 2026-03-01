# Go Rewrite — Incremental Migration Plan

## Goal

Incrementally rewrite `pyfixmsg_plus` in Go while keeping the Python system fully operational throughout. Each phase produces a tested, standalone Go module that replaces the equivalent Python subsystem. The Python codebase is retired phase-by-phase, not all at once.

## Guiding Principles

- **No big bang.** Every phase ends with working, tested code that provides value independently.
- **Interface-first.** Define Go interfaces before implementations; they are the contract between phases.
- **Parity before improvement.** Match Python behaviour exactly, then refactor. Do not chase performance or redesign during the initial port.
- **Shared persistence.** The SQLite message store schema is shared between Python and Go during the transition. Schema changes require both sides to migrate together.
- **Spec file compatibility.** QuickFIX XML spec files used by the Python library must parse identically in Go.

---

## Phase Overview

```
Phase 1  ──  FIX Message Core          (fixmsg package)
Phase 2  ──  Configuration & Crypto    (config, crypt packages)
Phase 3  ──  ID Generators             (idgen package)
Phase 4  ──  Message Store             (store package)
Phase 5  ──  Session State Machine     (session/state package)
Phase 6  ──  Network Layer             (network package)
Phase 7  ──  Message Handlers          (handler package)
Phase 8  ──  Heartbeat & Scheduler     (heartbeat, scheduler packages)
Phase 9  ──  FIX Engine Assembly       (engine package)
Phase 10 ──  CLI Tools                 (cmd/query)
Phase 11 ──  Integration & Python Sunset
```

---

## Phase 1 — FIX Message Core

**Scope:** Port `pyfixmsg/` — `FixFragment`, `FixMessage`, `RepeatingGroup`, `Codec` (wire-format parser/serialiser), and `FixSpec` (QuickFIX XML loader).

**Key decisions:**
- `FixMessage` is a `map[int]any` with header, body, and trailer ordering preserved via a sorted-tag writer.
- `FixFragment` is an alias for the same map type used inside repeating groups.
- `RepeatingGroup` is `[]FixFragment`.
- The codec must handle the `SOH` (`\x01`) delimiter and support configurable separators for test environments.
- `FixSpec` loads QuickFIX XML using `encoding/xml`; `lxml` acceleration has no Go equivalent—standard library is sufficient.

**Exit criteria:** All Python `pyfixmsg/` unit tests have Go equivalents that pass, including repeating group round-trips and checksum/length verification.

---

## Phase 2 — Configuration & Crypto

**Scope:** Port `ConfigManager` (singleton INI reader with encrypted value support) and `SimpleCrypt` (PBKDF2 + HMAC stream cipher).

**Key decisions:**
- `ConfigManager` is a thread-safe singleton using `sync.Once`.
- Encrypted config values retain the `ENC:<base64>` format for backward compatibility with the shared `config.ini`.
- `SimpleCrypt` uses `crypto/sha256` + PBKDF2 from `golang.org/x/crypto` to match Python's `hashlib.pbkdf2_hmac`.

**Exit criteria:** Python and Go can read the same `config.ini` and decrypt the same `ENC:` values identically.

---

## Phase 3 — ID Generators

**Scope:** Port `idgen/` — `NumericClOrdIdGenerator`, `YMDClOrdIdGenerator`, `MonthClOrdIdGenerator`, `NyseBranchSeqGenerator`, `OSESeqGenerator`, `CHIXBranchSeqGenerator`.

**Key decisions:**
- All generators implement a common `ClOrdIDGenerator` interface with `Encode(n int) string` and `Decode(s string) int`.
- No shared state; generators are safe for concurrent use after construction.

**Exit criteria:** Encode/decode round-trips match Python output for the same inputs.

---

## Phase 4 — Message Store

**Scope:** Port `DatabaseMessageStore` and `DatabaseMessageStoreAioSqlite` to a single Go implementation backed by `database/sql` + `mattn/go-sqlite3`.

**Key decisions:**
- Define a `MessageStore` interface (see `doc/spec/spec.md`).
- Use a single implementation; Go's `database/sql` is already non-blocking via connection pool. No need for two backends.
- The SQLite schema is identical to the Python schema (`messages` and `sessions` tables) to allow cohabitation during migration.
- All operations are protected by a `sync.Mutex` to match the Python `asyncio.Lock` semantics.

**Exit criteria:** Go store reads messages written by Python and vice versa against the same `.db` file.

---

## Phase 5 — Session State Machine

**Scope:** Port `state_machine.py` — states `Disconnected`, `Connecting`, `LogonInProgress`, `AwaitingLogon`, `Active`, `LogoutInProgress`, `Reconnecting`.

**Key decisions:**
- States implement a `State` interface with `OnEvent(event string, sm *StateMachine) State`.
- `StateMachine` is goroutine-safe; state transitions are serialised through a `sync.Mutex`.
- Subscribers receive state name strings via a registered callback slice, identical to Python.

**Exit criteria:** All state transition table tests pass, covering every (`currentState`, `event`) → `nextState` combination.

---

## Phase 6 — Network Layer

**Scope:** Port `network.py` — `NetworkConnection` base, `Initiator`, and `Acceptor`.

**Key decisions:**
- Use `net.Conn` and `crypto/tls` directly; no third-party networking library.
- `Initiator.Connect()` returns `(net.Conn, error)`; reconnect logic lives in the engine (Phase 9), not here.
- `Acceptor` uses `net.Listener` and spawns a goroutine per accepted connection, passing `net.Conn` to a handler factory — mirroring Python's `asyncio.start_server` + callback pattern.
- Buffer size default: 8192 bytes, matching Python.

**Exit criteria:** `Initiator` can connect to a QuickFIX acceptor and exchange raw bytes; `Acceptor` can accept a TCP connection and call a handler.

---

## Phase 7 — Message Handlers

**Scope:** Port `message_handler.py` — `MessageHandler` base, `MessageProcessor`, and all per-type handlers (`LogonHandler`, `HeartbeatHandler`, `TestRequestHandler`, `ResendRequestHandler`, `SequenceResetHandler`, `LogoutHandler`, etc.).

**Key decisions:**
- `MessageHandler` interface: `Handle(ctx context.Context, msg *fixmsg.FixMessage) error`.
- `MessageProcessor` dispatches by MsgType (tag 35) using a `map[string]MessageHandler`.
- Per-type handlers receive injected dependencies (store, state machine, application, engine) — no global state.
- All handlers log entry/exit at DEBUG level via a middleware wrapper (equivalent of `logging_decorator`).

**Exit criteria:** Each handler has unit tests using mock dependencies. Logon sequence (initiator and acceptor sides) is fully covered.

---

## Phase 8 — Heartbeat & Scheduler

**Scope:** Port `heartbeat.py`, `heartbeat_builder.py`, `testrequest.py`, and `scheduler.py`.

**Key decisions:**
- `Heartbeat` runs a single goroutine using `time.Ticker`; cancellation via `context.Context`.
- `HeartbeatBuilder` uses the builder pattern with method chaining.
- `Scheduler` parses the JSON schedule from config and runs `time.AfterFunc` tasks.

**Exit criteria:** Heartbeat sends at the configured interval; TestRequest is sent after a missed heartbeat; Scheduler fires actions at the correct times.

---

## Phase 9 — FIX Engine Assembly

**Scope:** Port `engine.py` — `FixEngine` struct, session lifecycle (`connect`, `logon`, `logout`, `disconnect`, reconnect with back-off), message send/receive loop, codec setup.

**Key decisions:**
- `FixEngine` is constructed via `NewFixEngine(cfg *config.ConfigManager, app Application) (*FixEngine, error)` — synchronous, no `create()` async factory.
- Reconnect uses exponential back-off up to `max_retries` (from config).
- The `Application` interface mirrors the Python abstract class exactly (see `doc/spec/spec.md`).
- Session ID format: `"{SENDER}-{TARGET}-{HOST}:{PORT}"` — identical to Python for store compatibility.

**Exit criteria:** End-to-end session test — Go initiator logs on to a Python (or QuickFIX) acceptor, exchanges heartbeats, and logs off cleanly.

---

## Phase 10 — CLI Tools

**Scope:** Port `tools/query.py` to `cmd/query/main.go`.

**Exit criteria:** `./query --config config.ini --session SENDER-TARGET` produces the same output as the Python CLI for the same database.

---

## Phase 11 — Integration & Python Sunset

- Remove Python dependencies from CI for subsystems fully replaced by Go.
- Update `copilot-instructions.md` to reflect Go as the primary language.
- Archive Python source under `_python_archive/` (do not delete; retain history).

---

## Coexistence Strategy

During phases 1–10, both Python and Go will be active. The integration boundary is the SQLite database:

```
Python FixEngine  ──writes──▶  fix_state.db  ◀──reads──  Go tools / tests
Go FixEngine      ──writes──▶  fix_state.db  ◀──reads──  Python tools / tests
```

Config file (`config.ini`) is read by both. The `ENC:` format for secrets must remain compatible.

---

## Go Module Layout

```
gorewrite/
├── go.mod                          (module: github.com/wxcuop/pyfixmsg_plus)
├── fixmsg/
│   ├── fixmessage.go
│   ├── fragment.go
│   ├── repeatinggroup.go
│   ├── codec/
│   │   └── stringfix.go
│   └── spec/
│       └── fixspec.go
├── fixengine/
│   ├── config/
│   │   └── configmanager.go
│   ├── crypt/
│   │   └── simplecrypt.go
│   ├── state/
│   │   └── statemachine.go
│   ├── network/
│   │   ├── initiator.go
│   │   └── acceptor.go
│   ├── store/
│   │   ├── store.go                (interface)
│   │   └── sqlite.go
│   ├── handler/
│   │   ├── handler.go              (interface + processor)
│   │   ├── logon.go
│   │   ├── heartbeat.go
│   │   └── ...
│   ├── heartbeat/
│   │   ├── heartbeat.go
│   │   └── builder.go
│   ├── scheduler/
│   │   └── scheduler.go
│   └── engine.go
├── idgen/
│   └── clordid.go
└── cmd/
    └── query/
        └── main.go
```
