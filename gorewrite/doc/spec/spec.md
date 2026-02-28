# Technical Specification — pyfixmsg_plus Go Rewrite

## Module

```
module github.com/wxcuop/pyfixmsg_plus

go 1.22
```

---

## Package Structure

```
gorewrite/
├── fixmsg/               FIX message core (FixMessage, Codec, FixSpec)
│   ├── codec/            Wire-format serialisation/deserialisation
│   └── spec/             QuickFIX XML specification loader
├── fixengine/
│   ├── config/           ConfigManager (INI, encryption)
│   ├── crypt/            SimpleCrypt (PBKDF2 + HMAC)
│   ├── state/            StateMachine and State types
│   ├── network/          Initiator, Acceptor (TCP/TLS)
│   ├── store/            MessageStore interface + SQLite implementation
│   ├── handler/          MessageHandler interface and per-type handlers
│   ├── heartbeat/        Heartbeat and HeartbeatBuilder
│   └── scheduler/        Scheduler
├── idgen/                ClOrdID generator suite
└── cmd/
    └── query/            CLI query tool
```

---

## Core Types

### `fixmsg` Package

```go
// FixFragment is the base map type for FIX tag-value pairs.
// Keys are integer tag numbers; values are strings (all FIX values are strings on the wire).
type FixFragment map[int]string

// FixMessage wraps FixFragment and carries codec and spec references needed
// for serialisation and repeating group resolution.
type FixMessage struct {
    FixFragment
    Codec *codec.Codec
    Spec  *spec.FixSpec  // nil if no spec loaded
}

// RepeatingGroup is an ordered list of FixFragments within a message.
type RepeatingGroup []FixFragment
```

**Key methods on `FixMessage`:**

```go
func (m *FixMessage) Get(tag int) (string, bool)
func (m *FixMessage) Set(tag int, value string)
func (m *FixMessage) MustGet(tag int) string              // panics if absent
func (m *FixMessage) Length() int                         // body length in bytes
func (m *FixMessage) Checksum() int                       // mod-256 sum
func (m *FixMessage) ToWire() ([]byte, error)             // serialise to FIX bytes
func (m *FixMessage) LoadFix(buf []byte) error            // parse from FIX bytes
func (m *FixMessage) FindAll(tag int) [][]any             // paths to tag in nested groups
func (m *FixMessage) Anywhere(tag int) bool
```

---

### `fixmsg/codec` Package

```go
type Codec struct {
    Spec          *spec.FixSpec  // nil disables repeating group parsing
    NoGroups      bool
    FragmentClass func() fixmsg.FixFragment  // factory for group members
    DecodeAs      string                     // "" = raw bytes, "utf-8" etc.
}

func (c *Codec) Parse(buf []byte, delimiter, separator byte) (*fixmsg.FixMessage, error)
func (c *Codec) Serialise(msg *fixmsg.FixMessage) ([]byte, error)
```

Constants:
```go
const (
    SOH       = '\x01' // standard FIX field separator
    Delimiter = '='
)
```

---

### `fixmsg/spec` Package

```go
type FixSpec struct {
    Version  string
    Tags     map[int]*FixTag
    Messages map[string]*MessageSpec
    Header   *ComponentSpec
    Trailer  *ComponentSpec
}

type FixTag struct {
    Name   string
    Number int
    Type   string
    Values map[string]string // enum value → name
}

func LoadSpec(filename string) (*FixSpec, error)
```

Header tags and their sort order are defined as package-level constants mirroring `pyfixmsg/reference.py`:
```go
var HeaderTags = []int{8, 9, 35, 1128, 1156, 1129, 49, 56, 115, 128, ...}
var TrailerTags = []int{93, 89, 10}
```

---

### `fixengine/config` Package

```go
type ConfigManager struct {
    // unexported fields; access via methods
}

// NewConfigManager returns the singleton. Safe to call from multiple goroutines.
func NewConfigManager(path string) (*ConfigManager, error)

func (c *ConfigManager) Get(section, key string) (string, bool)
func (c *ConfigManager) GetWithDefault(section, key, fallback string) string
func (c *ConfigManager) GetDecrypted(section, key string) (string, error) // handles ENC: prefix
func (c *ConfigManager) Set(section, key, value string)
func (c *ConfigManager) SetEncrypted(section, key, value string) error
func (c *ConfigManager) GetMessageStoreType() string
func (c *ConfigManager) Save() error
```

---

### `fixengine/crypt` Package

```go
type SimpleCrypt struct {
    // unexported; salt and PBKDF2 params set at construction
}

func NewSimpleCrypt(salt string) *SimpleCrypt
func (sc *SimpleCrypt) Encrypt(key []byte, plaintext string) (string, error) // returns base64
func (sc *SimpleCrypt) Decrypt(key []byte, ciphertext string) (string, error) // input base64
```

Encryption uses PBKDF2-HMAC-SHA256 (100,000 iterations) for key derivation and a deterministic stream cipher (XOR with key stream) verified by HMAC-SHA256, exactly matching `simple_crypt.py`.

---

### `fixengine/state` Package

```go
type State interface {
    OnEvent(event string, sm *StateMachine) State
    Name() string
}

// Concrete states
type Disconnected     struct{}
type Connecting       struct{}
type LogonInProgress  struct{}
type AwaitingLogon    struct{}
type Active           struct{}
type LogoutInProgress struct{}
type Reconnecting     struct{}

type StateMachine struct {
    // unexported; protected by sync.RWMutex
}

func NewStateMachine(initial State) *StateMachine
func (sm *StateMachine) OnEvent(event string)
func (sm *StateMachine) CurrentState() string
func (sm *StateMachine) Subscribe(fn func(stateName string))
```

**State transition table** (mirrors `state_machine.py`):

| Current State     | Event                           | Next State       |
|-------------------|---------------------------------|------------------|
| Disconnected      | `initiator_connect_attempt`     | Connecting       |
| Disconnected      | `client_accepted_awaiting_logon`| AwaitingLogon    |
| Disconnected      | `initiate_reconnect`            | Reconnecting     |
| Connecting        | `connection_established`        | LogonInProgress  |
| Connecting        | `connection_failed`/`disconnect`| Disconnected     |
| LogonInProgress   | `logon_successful`              | Active           |
| LogonInProgress   | `logon_failed`/`disconnect`     | Disconnected     |
| AwaitingLogon     | `logon_received_valid`          | Active           |
| AwaitingLogon     | `invalid_logon_received`/…      | Disconnected     |
| Active            | `logout_initiated`              | LogoutInProgress |
| Active            | `disconnect`/`force_disconnect` | Disconnected     |
| Active            | `initiate_reconnect`            | Reconnecting     |
| LogoutInProgress  | `logout_confirmed`/`disconnect` | Disconnected     |
| Reconnecting      | `connection_established`        | LogonInProgress  |
| Reconnecting      | `reconnect_failed_max_retries`  | Disconnected     |

---

### `fixengine/network` Package

```go
type NetworkConnection interface {
    Connect(ctx context.Context) error
    Send(data []byte) error
    Receive(ctx context.Context, handler func([]byte) error) error
    Disconnect() error
    IsRunning() bool
}

type Initiator struct {
    Host    string
    Port    int
    UseTLS  bool
    CACert  string // path to CA bundle for client-side TLS
    // unexported conn field
}

func NewInitiator(host string, port int, useTLS bool) *Initiator
func (i *Initiator) Connect(ctx context.Context) error
// Implements NetworkConnection

type Acceptor struct {
    Host     string
    Port     int
    UseTLS   bool
    CertFile string
    KeyFile  string
}

func NewAcceptor(host string, port int, useTLS bool) *Acceptor
// StartAccepting blocks; calls handlerFactory(conn net.Conn) in a new goroutine per client
func (a *Acceptor) StartAccepting(ctx context.Context, handlerFactory func(net.Conn)) error
func (a *Acceptor) Disconnect() error
```

---

### `fixengine/store` Package

```go
type MessageStore interface {
    Initialize(beginstring, sendercompid, targetcompid string) error
    StoreMessage(beginstring, sendercompid, targetcompid string, seqnum int, msg string) error
    GetMessage(beginstring, sendercompid, targetcompid string, seqnum int) (string, error)
    GetMessagesForResend(beginstring, sendercompid, targetcompid string, from, to int) ([]string, error)
    GetNextIncomingSeqNum() int
    GetNextOutgoingSeqNum() int
    IncrementIncomingSeqNum() error
    IncrementOutgoingSeqNum() error
    ResetSeqNums(incoming, outgoing int) error
    Close() error
}

// SQLiteMessageStore implements MessageStore.
// Schema is identical to the Python implementation for cross-compatibility.
type SQLiteMessageStore struct { /* unexported */ }

func NewSQLiteMessageStore(dbPath string) (*SQLiteMessageStore, error)
```

**SQLite Schema** (must not change without coordinating Python migration):
```sql
CREATE TABLE IF NOT EXISTS messages (
    beginstring  TEXT    NOT NULL,
    sendercompid TEXT    NOT NULL,
    targetcompid TEXT    NOT NULL,
    msgseqnum    INTEGER NOT NULL,
    message      TEXT    NOT NULL,
    timestamp    DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (beginstring, sendercompid, targetcompid, msgseqnum)
);

CREATE TABLE IF NOT EXISTS sessions (
    beginstring          TEXT    NOT NULL,
    sendercompid         TEXT    NOT NULL,
    targetcompid         TEXT    NOT NULL,
    creation_time        TEXT    NOT NULL,
    next_incoming_seqnum INTEGER NOT NULL,
    next_outgoing_seqnum INTEGER NOT NULL,
    PRIMARY KEY (beginstring, sendercompid, targetcompid)
);
```

---

### `fixengine/handler` Package

```go
type MessageHandler interface {
    Handle(ctx context.Context, msg *fixmsg.FixMessage) error
}

// HandlerDeps bundles all dependencies injected into handlers.
type HandlerDeps struct {
    Store       store.MessageStore
    StateMachine *state.StateMachine
    Application  Application
    Engine       EngineAPI  // interface, not concrete type — avoids import cycle
}

// MessageProcessor dispatches incoming messages by MsgType (tag 35).
type MessageProcessor struct {
    handlers map[string]MessageHandler
}

func NewMessageProcessor(deps HandlerDeps) *MessageProcessor
func (p *MessageProcessor) Register(msgType string, h MessageHandler)
func (p *MessageProcessor) Process(ctx context.Context, msg *fixmsg.FixMessage) error

// WithLogging wraps any MessageHandler with entry/exit DEBUG logging.
func WithLogging(h MessageHandler, logger *slog.Logger) MessageHandler
```

Per-type handlers (all in `handler` package):
- `LogonHandler`
- `HeartbeatHandler`
- `TestRequestHandler`
- `ResendRequestHandler`
- `SequenceResetHandler`
- `LogoutHandler`
- `RejectHandler`
- `ExecutionReportHandler`
- `NewOrderHandler`
- `CancelOrderHandler`
- `OrderCancelReplaceHandler`
- `OrderCancelRejectHandler`
- `NewOrderMultilegHandler`
- `MultilegOrderCancelReplaceHandler`

---

### `fixengine` Package — Application Interface

```go
// Application must be implemented by the library user.
// Mirrors the Python abstract base class exactly.
type Application interface {
    OnCreate(sessionID string)
    OnLogon(sessionID string)
    OnLogout(sessionID string)
    ToAdmin(msg *fixmsg.FixMessage, sessionID string)
    FromAdmin(msg *fixmsg.FixMessage, sessionID string)
    ToApp(msg *fixmsg.FixMessage, sessionID string)
    FromApp(msg *fixmsg.FixMessage, sessionID string)
    OnMessage(msg *fixmsg.FixMessage, sessionID string)
}
```

---

### `fixengine` Package — FixEngine

```go
type FixEngine struct {
    // All fields unexported; access via methods
}

func NewFixEngine(cfg *config.ConfigManager, app Application) (*FixEngine, error)

// Session lifecycle
func (e *FixEngine) Connect(ctx context.Context) error
func (e *FixEngine) Disconnect() error
func (e *FixEngine) StartAccepting(ctx context.Context) error // acceptor mode

// Message sending
func (e *FixEngine) SendMessage(msg *fixmsg.FixMessage) error
func (e *FixEngine) NewMessage(fields map[int]string) *fixmsg.FixMessage

// Accessors
func (e *FixEngine) SessionID() string
func (e *FixEngine) StateMachine() *state.StateMachine
```

**Session ID format:** `"{SENDER}-{TARGET}-{HOST}:{PORT}"` — identical to Python.

**Config keys read by FixEngine** (section `[FIX]`):

| Key | Default |
|-----|---------|
| `sender` | `SENDER` |
| `target` | `TARGET` |
| `version` | `FIX.4.4` |
| `spec_filename` | `FIX44.xml` |
| `host` | `127.0.0.1` |
| `port` | `5000` |
| `use_tls` | `false` |
| `mode` | `initiator` |
| `state_file` | `fix_state.db` |
| `heartbeat_interval` | `30` |
| `retry_interval` | `5` |
| `max_retries` | `5` |
| `message_store_type` | `database` |

---

### `fixengine/heartbeat` Package

```go
type Heartbeat struct { /* unexported */ }

type HeartbeatBuilder struct { /* unexported */ }

func NewHeartbeatBuilder() *HeartbeatBuilder
func (b *HeartbeatBuilder) SetSendCallback(fn func(*fixmsg.FixMessage) error) *HeartbeatBuilder
func (b *HeartbeatBuilder) SetConfigManager(cfg *config.ConfigManager) *HeartbeatBuilder
func (b *HeartbeatBuilder) SetInterval(seconds int) *HeartbeatBuilder
func (b *HeartbeatBuilder) SetStateMachine(sm *state.StateMachine) *HeartbeatBuilder
func (b *HeartbeatBuilder) SetEngine(e EngineAPI) *HeartbeatBuilder
func (b *HeartbeatBuilder) Build() *Heartbeat

func (h *Heartbeat) Start(ctx context.Context)
func (h *Heartbeat) Stop()
func (h *Heartbeat) UpdateLastReceived()
func (h *Heartbeat) UpdateLastSent()
```

---

### `fixengine/scheduler` Package

```go
type Schedule struct {
    Time   string `json:"time"`   // "HH:MM"
    Action string `json:"action"` // "start" | "stop" | "reset" | "reset_start"
}

type Scheduler struct { /* unexported */ }

func NewScheduler(cfg *config.ConfigManager, engine EngineAPI) (*Scheduler, error)
func (s *Scheduler) Start(ctx context.Context)
func (s *Scheduler) Stop()
```

---

### `idgen` Package

```go
type ClOrdIDGenerator interface {
    Encode(n int) string
    Decode(s string) int
}

func NewNumericClOrdIdGenerator(eid int, length int, seed bool) (*NumericClOrdIdGenerator, error)
func NewYMDClOrdIdGenerator(eid int, seed bool) *YMDClOrdIdGenerator
func NewMonthClOrdIdGenerator(eid int, seed bool) (*MonthClOrdIdGenerator, error)
func NewNyseBranchSeqGenerator(rangeStr, sep string) (*NyseBranchSeqGenerator, error)
func NewOSESeqGenerator(prefix string) (*OSESeqGenerator, error)
func NewCHIXBranchSeqGenerator(rangeStr string) (*CHIXBranchSeqGenerator, error)
```

---

## Error Handling

All public functions return `error` as the last return value. Errors are wrapped with context using `fmt.Errorf("fixengine/handler: logon: %w", err)`. Sentinel errors are defined as package-level `var`s:

```go
var (
    ErrInvalidMsgType     = errors.New("invalid or missing MsgType (tag 35)")
    ErrInvalidSeqNum      = errors.New("invalid or missing MsgSeqNum (tag 34)")
    ErrInvalidCompID      = errors.New("CompID mismatch in logon")
    ErrStoreNotInitialized = errors.New("message store not initialized")
    ErrNotConnected       = errors.New("not connected")
)
```

---

## Logging

Use `log/slog` (standard library, Go 1.21+). Default level is `INFO`. All packages obtain a named logger:

```go
logger := slog.With("component", "LogonHandler")
```

Debug-level logs use structured key-value pairs:
```go
logger.Debug("handling message", "msgType", msgType, "seqNum", seqNum)
```

---

## Concurrency Model

- The Python `asyncio` event loop maps to Go goroutines + channels.
- The receive loop runs in a dedicated goroutine spawned by `FixEngine.Connect`.
- The heartbeat loop runs in a dedicated goroutine spawned by `Heartbeat.Start`.
- All shared state (store, state machine) is protected by `sync.Mutex` or `sync.RWMutex`.
- `context.Context` is threaded through all long-running operations for clean cancellation.
- Message sending is serialised through a `sync.Mutex` on the writer to prevent interleaved writes.

---

## Testing Strategy

- **Unit tests**: every package has a `_test.go` file using only standard `testing` package + `testify/assert`.
- **Integration tests**: in `fixengine/engine_integration_test.go`, tagged `//go:build integration`.
- **Table-driven tests**: preferred for state machine transitions, codec round-trips, and ID generator encode/decode.
- **Mocks**: hand-written interfaces; no mock generation framework unless added explicitly.
- Run unit tests: `go test ./...`
- Run with integration: `go test -tags integration ./...`
- Run a single test: `go test ./fixengine/handler/ -run TestLogonHandler_AcceptorSide`
