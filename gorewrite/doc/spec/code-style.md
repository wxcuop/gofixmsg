# Go Code Style — pyfixmsg_plus

This document defines the code style conventions for the Go rewrite. All contributions must follow these guidelines. Where Go idioms conflict with direct Python port fidelity, Go idioms win.

---

## Formatting

- **`gofmt` is mandatory.** All code must be formatted with `gofmt` before committing. CI will reject unformatted code.
- **`goimports`** is preferred over bare `gofmt` — it also organises imports into stdlib / external / internal groups.
- Line length is not hard-limited but keep lines under ~120 characters for readability.

---

## Naming

### Packages
- Lowercase, single word: `fixmsg`, `codec`, `spec`, `config`, `crypt`, `state`, `network`, `store`, `handler`, `heartbeat`, `scheduler`, `idgen`.
- No underscores, no `fixmsg_` prefixes inside the package itself.
- Package names are the last segment of the import path; avoid stutter (`codec.Codec` not `codec.CodecImpl`).

### Types
- `PascalCase` for all exported types.
- Acronyms: `FixMessage`, `TLSConfig`, `SQLiteMessageStore`, `ClOrdID`. Match the domain language precisely.

### Interfaces
- Name by behaviour, not by implementation: `MessageStore`, `MessageHandler`, `NetworkConnection`, `Application`.
- One-method interfaces take the method name + `-er`: `Sender`, `Receiver`, `Connector` — only if truly general-purpose. Prefer descriptive names for domain interfaces.

### Variables and Parameters
- `camelCase` for unexported, `PascalCase` for exported.
- Avoid single-letter names except for loop indices (`i`, `j`) and short-lived error values (`err`).
- Receiver names: short, consistent, lowercase abbreviation of the type. `func (sm *StateMachine)`, `func (h *LogonHandler)`, `func (e *FixEngine)`.

### Constants
- Exported constants: `PascalCase`. `SOH`, `DefaultHeartbeatInterval`, `MaxRetries`.
- Iota-based enums: use a type alias. `type SessionMode int`.

---

## Error Handling

- **Never ignore errors.** Every `error` return must be checked or explicitly discarded with a comment.
- Wrap errors with context at package boundaries:
  ```go
  return fmt.Errorf("handler/logon: validate compids: %w", err)
  ```
- Define sentinel errors as package-level `var`s, not inline `errors.New`:
  ```go
  var ErrInvalidSeqNum = errors.New("invalid sequence number")
  ```
- Do not use `panic` for control flow. Reserve `panic` for programmer errors (nil pointer, impossible state). Recover only at the top-level goroutine boundary.
- Return early on error; avoid deep nesting:
  ```go
  // Preferred
  if err != nil {
      return fmt.Errorf("...: %w", err)
  }
  // Not preferred: else branch after return
  ```

---

## Interfaces and Dependency Injection

- Depend on interfaces, not concrete types. Constructor parameters for collaborators must be interface types.
- Define interfaces in the **consuming** package, not the providing package (Go convention):
  ```go
  // In handler package, define the store interface it needs:
  type messageStore interface {
      GetNextIncomingSeqNum() int
      // only the methods this handler uses
  }
  ```
- Prefer small interfaces. A handler that only reads from the store should not receive the full `MessageStore` interface.

---

## Structs and Constructors

- All struct fields that represent dependencies or configuration should be **unexported**.
- Provide a constructor function `New<Type>(...)` for every exported struct. Do not allow zero-value construction of structs with required dependencies.
  ```go
  func NewLogonHandler(deps HandlerDeps) *LogonHandler {
      return &LogonHandler{
          store:        deps.Store,
          stateMachine: deps.StateMachine,
          app:          deps.Application,
          logger:       slog.With("component", "LogonHandler"),
      }
  }
  ```
- Fluent builders (e.g., `HeartbeatBuilder`) return the builder pointer for chaining, and `Build()` returns the built value plus an `error`.

---

## Concurrency

- Document goroutine ownership: each goroutine must have a clear owner responsible for stopping it.
- Use `context.Context` for cancellation on all long-running operations. Never use a bare `time.Sleep` in a loop that should be cancellable.
  ```go
  select {
  case <-ctx.Done():
      return ctx.Err()
  case <-ticker.C:
      // do work
  }
  ```
- Protect shared state with `sync.Mutex` (write-heavy) or `sync.RWMutex` (read-heavy). Lock as narrowly as possible.
- Do not share channels between unrelated components. Prefer explicit callbacks (function values) over channels for event notification, matching the Python subscriber pattern.
- Avoid `init()` that starts goroutines.

---

## Logging

- Use `log/slog` exclusively. No `fmt.Println` in production paths.
- Obtain a component-scoped logger at construction time:
  ```go
  logger: slog.With("component", "FixEngine", "session", sessionID)
  ```
- Log at the appropriate level:
  - `DEBUG`: per-message entry/exit, state transitions, raw bytes.
  - `INFO`: session lifecycle events (logon, logout, reconnect).
  - `WARN`: recoverable anomalies (sequence gaps, missed heartbeat).
  - `ERROR`: unrecoverable errors before returning.
- Structured logging — use key-value pairs, never string interpolation in the message:
  ```go
  // Correct
  logger.Debug("received message", "msgType", msgType, "seqNum", seqNum)
  // Incorrect
  logger.Debug(fmt.Sprintf("received message: type=%s seq=%d", msgType, seqNum))
  ```

---

## Comments and Documentation

- All exported types, functions, and methods must have a `// TypeName ...` GoDoc comment.
- Comments explain *why*, not *what* (the code shows what).
- Mark FIX protocol field references in comments:
  ```go
  // MsgSeqNum is FIX tag 34.
  seqNum := msg.MustGet(34)
  ```
- Do not copy Python docstrings verbatim; rewrite in idiomatic Go style.

---

## Testing

- Test files live alongside source: `handler/logon_test.go`.
- Use `testify/assert` and `testify/require` for assertions. `require` on fatal setup, `assert` on individual checks.
- Table-driven tests for all state transitions, codec round-trips, and ID generators:
  ```go
  tests := []struct {
      name     string
      input    string
      expected int
  }{...}
  for _, tc := range tests {
      t.Run(tc.name, func(t *testing.T) { ... })
  }
  ```
- Integration tests are in files ending `_integration_test.go` and gated with `//go:build integration`.
- Run a single test: `go test ./fixengine/handler/ -run TestLogonHandler`
- Benchmarks use `func BenchmarkXxx(b *testing.B)` and live in `_test.go` files.

---

## Imports

Three groups, separated by blank lines:
```go
import (
    // stdlib
    "context"
    "fmt"
    "sync"

    // external
    "github.com/stretchr/testify/assert"
    "golang.org/x/crypto/pbkdf2"

    // internal
    "github.com/wxcuop/pyfixmsg_plus/fixmsg"
    "github.com/wxcuop/pyfixmsg_plus/fixengine/state"
)
```

---

## File Layout

Within a `.go` file, order sections as follows:
1. `package` declaration + GoDoc package comment
2. `import` block
3. Constants (`const`)
4. Type declarations (`type`)
5. Variables (`var`) — sentinel errors, package-level singletons
6. Constructor functions (`New...`)
7. Methods on the primary type
8. Methods on secondary types
9. Unexported helper functions

---

## What to Avoid

- **`init()` functions** — except for `log/slog` global default setup, if needed.
- **Global mutable state** — `ConfigManager` is the single permitted singleton, via `sync.Once`.
- **Type assertions without ok check** — always use the two-value form: `v, ok := x.(T)`.
- **Empty `interface{}`** — use `any` (Go 1.18+ alias) or a typed interface where possible.
- **Named return values** — avoid except where they materially improve clarity (e.g., deferred error capture).
- **`recover()` in library code** — only at goroutine entry points in the engine.
