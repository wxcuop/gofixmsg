# GEMINI.md - Project Context & Instructions

## Project Overview
`pyfixmsg_plus` is a Python-based FIX (Financial Information eXchange) engine. It aims to provide a robust, asynchronous, and extensible implementation of the FIX protocol. **A primary long-term goal is to rewrite the core engine in Go (`gorewrite/`) for improved performance and concurrency.**

## Technical Stack
- **Language:** Python 3.x
- **Asynchronous Framework:** `asyncio`
- **Database:** SQLite (via `aiosqlite` and `sqlite3`)
- **Testing:** `pytest`
- **Configuration:** `configparser` (`.ini` files)

## Core Components
- `pyfixmsg_plus.fixengine.engine`: The main FIX engine coordinator.
- `pyfixmsg_plus.fixengine.network`: Handles TCP connections (acceptor/initiator).
- `pyfixmsg_plus.fixengine.state_machine`: Manages the FIX session state.
- `pyfixmsg_plus.fixengine.database_message_store`: Persists messages for recovery.
- `pyfixmsg_plus.fixengine.message_handler`: Routes and processes incoming messages.

## Development Standards
- **Asynchronous Programming:** Use `async`/`await` for all I/O bound operations.
- **Error Handling:** Use custom exceptions where appropriate. Ensure graceful session termination on critical errors.
- **Logging:** Use the standard `logging` module. Avoid `print` statements.
- **Testing:**
    - New features MUST include unit tests in `tests/unit`.
    - Integration tests should be placed in `tests/integration`.
    - Run tests using `pytest`.
- **Style:** Follow PEP 8. Use clear, descriptive variable and function names.

## Key Files & Directories
- `pyfixmsg_plus/`: Core library code.
- `tests/`: Test suite.
- `examples/`: Sample applications using the engine.
- `gorewrite/`: A Go implementation of the engine (experimental/parallel project).

## Mandates
- **Surgical Updates:** When modifying existing code, maintain the existing style and structure.
- **Verification:** Always run `pytest` after making changes to ensure no regressions.
- **Documentation:** Update docstrings for any modified or new functions/classes.
