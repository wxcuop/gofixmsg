# GoFixMsg

A high-performance FIX (Financial Information eXchange) protocol implementation written in Go.

## Overview

GoFixMsg is a complete rewrite of the FIX session management library, providing robust FIX protocol support with:

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

## Legacy Python Code

The original Python implementation is archived in `zz_archive/`. It remains fully functional but is no longer actively maintained. The Go rewrite is the recommended implementation for new projects.

## Documentation

- [Architecture & Design](gofixmsg/doc/plan.md) - High-level system design
- [Phase 31/32 Plan](gofixmsg/doc/PHASE_31_32_PLAN.md) - Development roadmap
- [FIX Protocol Spec Integration](gofixmsg/doc/spec/) - QuickFIX XML spec handling

## License

See LICENSE.md for licensing information.
