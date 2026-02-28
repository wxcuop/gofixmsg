# SOLID Principles — pyfixmsg_plus Go Design

This document maps each SOLID principle to concrete, enforceable design decisions in this codebase. Every section includes a rule, a rationale, and examples drawn directly from the package structure.

---

## S — Single Responsibility Principle

> A type or package should have one reason to change.

### Rules

- **Each package owns one domain concern.** `fixmsg` parses and represents FIX messages. `fixengine/store` persists them. `fixengine/handler` processes them. `fixengine/state` tracks session state. None of these concerns bleeds into another package.
- **`FixEngine` orchestrates; it does not implement.** The engine struct holds references to collaborators (store, state machine, network, processor) and calls them. It does not contain parsing logic, SQL queries, or heartbeat scheduling directly.
- **Message handlers handle one message type each.** `LogonHandler` handles `MsgType=A`. It does not also handle heartbeats. Each handler has one reason to change: the FIX protocol behaviour of its message type.

### Example: What NOT to do

```go
// Bad: FixEngine directly queries the database
func (e *FixEngine) getSeqNum() int {
    row := e.db.QueryRow("SELECT next_outgoing_seqnum FROM sessions WHERE ...")
    // ...
}
```

```go
// Good: FixEngine delegates to the store interface
func (e *FixEngine) getSeqNum() int {
    return e.store.GetNextOutgoingSeqNum()
}
```

---

## O — Open/Closed Principle

> Software entities should be open for extension, closed for modification.

### Rules

- **Add new message types by registering a new handler, not by modifying `MessageProcessor`.** The processor dispatches by MsgType string from a `map[string]MessageHandler`. Adding support for a new FIX message type (e.g., `MsgType=AE` for TradeCaptureReport) means implementing `MessageHandler` and calling `processor.Register("AE", handler)` — zero changes to existing code.
- **Add new store backends by implementing `MessageStore`.** If a future requirement needs an in-memory store (for testing) or a PostgreSQL store, implement the interface. `FixEngine` never changes.
- **Add new network transports by implementing `NetworkConnection`.** A WebSocket or FIXT transport can be added without touching the engine.

### Example

```go
// MessageProcessor is closed for modification
type MessageProcessor struct {
    handlers map[string]MessageHandler
}

// Open for extension: register any handler without touching MessageProcessor
processor.Register("AE", NewTradeCaptureReportHandler(deps))
processor.Register("X",  NewMarketDataIncrRefreshHandler(deps))
```

### Extension points (interfaces, not concrete types)

| Extension Point | Interface |
|-----------------|-----------|
| New message type | `MessageHandler` |
| New persistence backend | `MessageStore` |
| New network transport | `NetworkConnection` |
| New application callback | `Application` |
| New ClOrdID scheme | `ClOrdIDGenerator` |

---

## L — Liskov Substitution Principle

> Subtypes must be substitutable for their base types without altering the correctness of the program.

### Rules

- **All `MessageStore` implementations must honour the same contract.** `SQLiteMessageStore` and any future in-memory or mock store must: (a) return the same sequence numbers after `Initialize` as were last persisted, (b) never return a stored message that was not passed to `StoreMessage`, (c) return `ErrStoreNotInitialized` if called before `Initialize`. Tests must validate this contract against every implementation.
- **All `State` implementations must behave predictably.** Every state's `OnEvent` must return either a new state (transition) or itself (no-op). It must never return `nil`. Returning `nil` would break `StateMachine.OnEvent` — this is a Liskov violation.
- **All `ClOrdIDGenerator` implementations must round-trip.** For any valid `n`, `Decode(Encode(n)) == n`. Violating this contract breaks order tracking regardless of which generator is in use.

### Enforcement

Define contract tests as shared test functions that any implementation can run:

```go
// In store/contract_test.go
func RunMessageStoreContractTests(t *testing.T, store MessageStore) {
    t.Run("round-trip store and retrieve", func(t *testing.T) { ... })
    t.Run("sequence numbers persist", func(t *testing.T) { ... })
    t.Run("returns error before Initialize", func(t *testing.T) { ... })
}

// In store/sqlite_test.go
func TestSQLiteStoreContract(t *testing.T) {
    s, _ := NewSQLiteMessageStore(":memory:")
    RunMessageStoreContractTests(t, s)
}
```

---

## I — Interface Segregation Principle

> Clients should not be forced to depend on interfaces they do not use.

### Rules

- **Define interfaces in the consuming package, not the providing package.** Each consumer declares only the methods it needs. This avoids large, monolithic interfaces.
- **`MessageHandler` depends on a narrow `storeReader`, not the full `MessageStore`.** A `LogonHandler` only needs to read sequence numbers; it does not need `StoreMessage` or `GetMessagesForResend`. Define a local interface:

```go
// In handler package — only what LogonHandler needs from the store
type seqNumReader interface {
    GetNextIncomingSeqNum() int
    GetNextOutgoingSeqNum() int
    ResetSeqNums(incoming, outgoing int) error
}
```

- **`FixEngine`'s internal collaborators use narrow interfaces.** The heartbeat mechanism does not receive the full `FixEngine`; it receives an `engineAPI` interface with only `SendMessage` and `StateMachine()`.

```go
// In heartbeat package
type engineAPI interface {
    SendMessage(msg *fixmsg.FixMessage) error
    StateMachine() *state.StateMachine
}
```

- **`Application` is the user-facing interface.** It has all eight callbacks because the user controls the implementation — they will implement what they need and leave others empty. This is intentional and acceptable at the API boundary.

### Anti-pattern to avoid

```go
// Bad: ResendRequestHandler depends on everything
type ResendRequestHandler struct {
    store store.MessageStore // includes StoreMessage, ResetSeqNums, Close — not needed here
}

// Good: ResendRequestHandler depends only on what it uses
type messageReplayer interface {
    GetMessagesForResend(beginstring, sendercompid, targetcompid string, from, to int) ([]string, error)
}
```

---

## D — Dependency Inversion Principle

> High-level modules should not depend on low-level modules. Both should depend on abstractions.

### Rules

- **`FixEngine` (high-level) depends on interfaces, never on `SQLiteMessageStore` (low-level) directly.** The engine holds a `store.MessageStore` interface. The concrete SQLite implementation is injected by the application at startup.
- **Constructor injection is the only permitted form of dependency injection.** Dependencies are passed into `New...` constructors as interface arguments. No service locator, no global registry, no `init()` wiring.
- **`MessageStoreFactory` is the composition root for the store.** It reads the `message_store_type` config key and returns a `MessageStore` interface. The caller never names a concrete type:

```go
// Composition root (main or engine constructor)
storeType := cfg.GetMessageStoreType()
ms, err := store.NewFromConfig(storeType, dbPath, beginstring, sendercompid, targetcompid)
if err != nil {
    return nil, fmt.Errorf("NewFixEngine: create store: %w", err)
}
engine := &FixEngine{store: ms, ...}
```

- **`FixEngine` does not import `fixengine/store` directly.** It imports the `store.MessageStore` interface type. The concrete package is only imported in the composition root (`engine.go`'s factory function) where wiring happens.

### Dependency graph (direction of arrows = depends on)

```
cmd/query  ──────────────────────────────────────────▶  store.MessageStore (interface)
                                                              ▲
fixengine (FixEngine) ──────────▶  store.MessageStore         │
                       ──────────▶  state.StateMachine         │
                       ──────────▶  network.NetworkConnection   │
                       ──────────▶  handler.MessageProcessor    │
                                                              │
store/sqlite.go ─────────────────────────────────────▶ implements MessageStore
```

No arrows point from low-level packages (`store/sqlite`) back up to `fixengine`. The dependency graph is acyclic.

---

## Summary Table

| Principle | Primary enforcement mechanism |
|-----------|-------------------------------|
| **S** – Single Responsibility | One domain concern per package; `FixEngine` orchestrates only |
| **O** – Open/Closed | `MessageProcessor` dispatch map; all extension points are interfaces |
| **L** – Liskov Substitution | Contract tests run against every `MessageStore` and `State` implementation |
| **I** – Interface Segregation | Consumer-defined narrow interfaces; `HandlerDeps` uses minimal interface slices |
| **D** – Dependency Inversion | Constructor injection only; `FixEngine` depends on `store.MessageStore`, never on `SQLiteMessageStore` |
