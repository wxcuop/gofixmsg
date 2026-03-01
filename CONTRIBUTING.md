# Contributing to GoFixMsg

Thank you for your interest in contributing to GoFixMsg! This document outlines guidelines for contributing to the Go implementation of the FIX protocol library.

## Code Organization

GoFixMsg is organized into focused packages:

- **engine/** - Core session management and FIX engine implementation
- **fixmsg/** - FIX message parsing, serialization, codec
- **handler/** - Message handler registration and dispatch
- **network/** - Network connections (Initiator/Acceptor)
- **store/** - Message persistence (SQLite)
- **state/** - Session state machine
- **heartbeat/** - Heartbeat and test request logic
- **scheduler/** - Scheduled session actions
- **integration/** - End-to-end integration tests

## Before You Start

1. **Review existing code**: Familiarize yourself with the package structure and coding patterns
2. **Check open issues/discussions**: Your idea might already be under discussion
3. **Follow Go conventions**: Use `go fmt`, `go vet`, and write idiomatic Go code
4. **Write tests**: All new features require unit and/or integration tests

## Development Workflow

1. **Create a feature branch**: `git checkout -b feature/your-feature`
2. **Make changes**: Keep commits focused and well-documented
3. **Test locally**:
   ```bash
   go test ./... -v
   ```
4. **Lint and format**:
   ```bash
   go fmt ./...
   go vet ./...
   ```
5. **Commit with clear messages**: Reference any related issues

## Testing Guidelines

- **Unit tests** go in `*_test.go` files alongside the code being tested
- **Integration tests** go in `integration/` for full end-to-end scenarios
- **Coverage**: Aim for high coverage; use `go test -cover ./...`
- **Test markers**: Use `t.Parallel()` where appropriate for faster parallel test execution

## Code Style

- Follow standard Go conventions and the [Effective Go](https://golang.org/doc/effective_go) guide
- Use clear, descriptive variable and function names
- Document exported types, functions, and constants with comment strings
- Keep functions focused and testable
- Use interfaces for abstraction where it adds value

## Submitting Changes

1. **Push your branch** to your fork
2. **Open a pull request** with a clear description of:
   - What problem it solves or feature it adds
   - How it was tested
   - Any breaking changes (if applicable)
3. **Respond to review feedback** promptly

## Commit Message Format

```
[component] Brief summary

Detailed explanation of the change and why it was needed.
Include any fixes or references to issues.

Fixes: #123
```

Example:
```
[engine] Add support for reconnect backoff strategy

Implements exponential backoff with jitter for session reconnection attempts.
Allows configuration via backoff_strategy config parameter.

Fixes: #45
```

## Adding New Message Handlers

1. Create a new handler type in `handler/handler.go` or a new file
2. Implement the `async def handle()` method signature
3. Register the handler in `engine/engine.go` handler map
4. Add unit tests in `handler/*_test.go`
5. Add integration test in `integration/`

Example:
```go
type CustomHandler struct {
    engine *Engine
}

func (h *CustomHandler) Handle(msg *fixmsg.FixMessage) error {
    // Handle message logic
    return nil
}
```

## Reporting Issues

- **Bug report**: Include steps to reproduce, expected vs actual behavior, Go version
- **Feature request**: Describe the use case and desired behavior
- **Documentation**: Suggest improvements or point out unclear sections

## Questions?

- Check the [documentation](gofixmsg/doc/)
- Review [integration tests](integration/) for usage examples
- Look at the [examples](examples/) for sample applications

## Licensing

By contributing to GoFixMsg, you agree that your contributions will be licensed under the same license as the project. See LICENSE.md.

Thank you for helping make GoFixMsg better!
