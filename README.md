# GoFixMsg

A FIX (Financial Information eXchange) protocol implementation written in Go.

- https://wxcuop.github.io/gofixmsg/godoc/index.html

## Overview

GoFixMsg is an implmentation of the FIX session management library, providing support for:

- **async-first architecture** using goroutines for concurrent session handling
- **state machine-based session management** (Disconnected → Connecting → AwaitingLogon → LoggedOn)
- **flexible message handlers** for custom FIX message processing
- **persistent message stores** with SQLite support
- **TLS/SSL support** for secure connections
- **heartbeat and test request management**
- **multi-session support** with configurable scheduling

## Directory Structure

- `engine/` - Core FIX engine and state machine
- `fixmsg/` - FIX message parsing, serialization, and codec
- `handler/` - Message handler framework
- `network/` - Network connection management (Initiator/Acceptor)
- `store/` - Message persistence layer
- `state/` - Session state management
- `heartbeat/` - Heartbeat and test request logic
- `scheduler/` - Scheduled tasks for session actions
- `integration/` - Integration tests
- `examples/` - Example applications

## Requirements

- Go 1.21 or later
- SQLite3 (for message persistence)

## Building

```bash
cd gofixmsg
go build ./...
```

## Running Tests

```bash
cd gofixmsg
go test ./... -v
```

## Configuration

Configuration is managed via `.ini` files. See `examples/config.ini` for an example configuration with:

- Connection settings (host, port, sender/target IDs)
- Message store configuration
- Heartbeat intervals
- TLS settings

## QuickStart

See `examples/` for complete initiator and acceptor applications demonstrating:

- Creating a FIX engine
- Connecting as initiator or acceptor
- Sending and receiving messages
- Handling session callbacks

## Documentation

- [Architecture & Design](gofixmsg/doc/plan.md) - High-level system design
- [FIX Protocol Spec Integration](gofixmsg/doc/spec/) - QuickFIX XML spec handling

## License

See LICENSE.md for licensing information.
