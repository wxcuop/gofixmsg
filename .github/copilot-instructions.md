# Copilot Instructions for pyfixmsg_plus

## Build, Test, and Lint

**Install dependencies:**
```bash
pip install -r requirements.txt
pip install pytest pytest-timeout pytest-asyncio pytest-cov faker
```

**Run all tests** (requires a QuickFIX spec file):
```bash
pytest --spec=FIX44.xml --timeout=90
```
The FIX44.xml spec file is not bundled; download it from https://github.com/quickfix/quickfix/tree/master/spec or fetch it during CI:
```bash
curl https://raw.githubusercontent.com/quickfix/quickfix/master/spec/FIX44.xml -o FIX44.xml
```

**Run a single test file or function:**
```bash
pytest tests/unit/test_state_machine_core.py -v
pytest tests/unit/test_engine_core.py::TestEngineCore::test_some_function -v
```

**Run slow or chaos tests** (skipped by default):
```bash
pytest --run-slow --run-chaos
```

**Run tests by marker:**
```bash
pytest -m unit
pytest -m integration
pytest -m "not slow"
```

**Lint:**
```bash
pylint pyfixmsg_plus/
```
Max line length is 120; `no-member`, `star-args`, and `bad-continuation` are disabled in `.pylintrc`.

**Coverage is enforced at 95%** via `pytest.ini`. Reports are written to `htmlcov/` and `coverage.xml`.

## Architecture Overview

The repo contains two distinct layers:

### 1. `pyfixmsg/` — Core FIX message library (forked from Morgan Stanley)
- `FixMessage` (inherits `dict`) is the central class for parsing, manipulating, and serialising FIX messages.
- `Codec` (`pyfixmsg/codecs/stringfix.py`) handles wire-format parsing and serialisation.
- `FixSpec` (`pyfixmsg/reference.py`) loads QuickFIX XML spec files; required only for repeating group support.

### 2. `pyfixmsg_plus/` — FIX session management layer built on top of `pyfixmsg`
The session engine is async (asyncio) and structured as follows:

| Component | Role |
|---|---|
| `FixEngine` (`fixengine/engine.py`) | Central coordinator. Created with `await FixEngine.create(config, app)`. Supports `initiator` and `acceptor` modes. |
| `Application` (`application.py`) | Abstract base class users must subclass. Callbacks: `onCreate`, `onLogon`, `onLogout`, `toAdmin`, `fromAdmin`, `toApp`, `fromApp`, `onMessage`. |
| `ConfigManager` (`fixengine/configmanager.py`) | Singleton. Reads `config.ini`. Sensitive values stored as `ENC:<base64>` using `SimpleCrypt`. |
| `StateMachine` (`fixengine/state_machine.py`) | State: `Disconnected → Connecting → AwaitingLogon → LoggedOn → ...`. Subscribers receive state-name strings on transitions. |
| `MessageProcessor` / `MessageHandler` (`fixengine/message_handler.py`) | Per-MsgType handlers registered by FIX type code (e.g. `'A'`=Logon, `'D'`=NewOrder). All handlers are async and decorated with `logging_decorator`. |
| `NetworkConnection` (`fixengine/network.py`) | `Acceptor` and `Initiator` subclasses wrapping asyncio streams with optional TLS. |
| `MessageStoreFactory` (`fixengine/message_store_factory.py`) | Creates either `DatabaseMessageStore` (sync sqlite3) or `DatabaseMessageStoreAioSqlite` (async aiosqlite) based on `message_store_type` config key. |
| `Heartbeat` / `HeartbeatBuilder` | Built via fluent builder. Manages heartbeat tasks and test request tracking. |
| `Scheduler` | Parses JSON schedule from config (`[Scheduler] schedules`) and calls engine actions at specified times. |

## Key Conventions

### Async-first engine
All engine operations are async. Use `await FixEngine.create(...)` (not the constructor directly) to get a fully-initialized engine.

### Message handler registration
Handlers are registered by FIX MsgType string in `engine.py`:
```python
handler_classes = {'A': LogonHandler, 'D': NewOrderHandler, ...}
```
To add a new message type, create a subclass of `MessageHandler`, implement `async def handle(self, message)`, and register it in this dict.

### `logging_decorator`
All `MessageHandler.handle()` methods should be decorated with `@logging_decorator` for consistent debug logging of incoming message type and sequence number.

### ConfigManager is a Singleton
`ConfigManager` uses `__new__` to enforce a single instance per process. In tests, reset state carefully or use a `temp_config_file` fixture (provided in `tests/fixtures/test_fixtures.py`).

### Encrypted config values
Config values prefixed with `ENC:` are decrypted automatically when `get(..., decrypt_value=True)` is called. Use `configmanager.encrypt(value)` / `configmanager.decrypt(value)` module-level helpers.

### Message store dual backends
`DatabaseMessageStore` uses `sqlite3` synchronously with an `asyncio.Lock`. `DatabaseMessageStoreAioSqlite` uses `aiosqlite` fully async. Both must be `await store.initialize()`-d before use.

### Test markers
- `@pytest.mark.slow` and `@pytest.mark.chaos` tests are skipped unless `--run-slow` / `--run-chaos` flags are passed.
- `@pytest.mark.asyncio` (or `asyncio_mode=auto` in `pytest.ini`) applies to all async test functions automatically.

### FIX tag access
`FixMessage` keys are integers (tag numbers). Access values by integer tag: `msg.get(35)` for MsgType, `msg.get(49)` for SenderCompID, etc.

### Session ID format
Session IDs are formatted as `"{SENDER}-{TARGET}-{HOST}:{PORT}"` (see `engine.py`).
