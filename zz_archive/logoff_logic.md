# FIX Logoff Logic in pyfixmsg_plus

This document explains how the FIX logoff handshake is implemented and managed in the `pyfixmsg_plus` engine and application.

---

## Overview

The FIX protocol requires a two-way logoff handshake:
1. One side sends a Logoff message (`35=5`).
2. The counterparty responds with its own Logoff message (`35=5`).
3. Both sides then disconnect the session.

In `pyfixmsg_plus`, this handshake is encapsulated in the `FixEngine` class, so application code does not need to manage protocol details.

---

## Logoff Flow

### 1. Application Requests Logoff

The application initiates a logoff by calling:

```python
await engine.request_logoff(timeout=10)
```

- This sends a FIX Logoff message to the counterparty.
- The engine then waits for a Logoff response, up to the specified timeout.

---

### 2. Engine Waits for Logoff Response

- The engine sets up an internal `Future` to wait for the Logoff response.
- When a Logoff message is received from the counterparty, the engine's `LogoutHandler` calls `engine.notify_logoff_received()`, which completes the `Future`.

---

### 3. Engine Disconnects

- Once the Logoff response is received (or the timeout expires), the engine disconnects the network connection and cleans up resources.
- If the session is already disconnected, the disconnect call is a no-op.

---

## Key Code Components

### Application Side

The application only needs to call:

```python
await engine.request_logoff(timeout=10)
```

No manual waiting or state tracking is required.

---

### Engine Side

**In `FixEngine`:**

```python
async def request_logoff(self, timeout: float = 10.0):
    # Send Logoff, wait for response, then disconnect
    await self.send_logout_message("Operator requested logout")
    try:
        await asyncio.wait_for(self._logoff_future, timeout=timeout)
    except asyncio.TimeoutError:
        self.logger.warning("Timeout waiting for Logoff response.")
    await self.disconnect(graceful=False)
```

**In `LogoutHandler`:**

```python
def notify_logoff_received(self):
    # Called when a Logoff is received from the counterparty
    if self._logoff_future and not self._logoff_future.done():
        self._logoff_future.set_result(True)
```

---

## Why `graceful=False` on Disconnect?

- After the handshake, the session is already logged off.
- `graceful=False` means "just close the connection, don't try to send another Logoff."
- This ensures a clean and immediate disconnect.

---

## Summary

- The logoff handshake is handled entirely by the engine.
- The application simply calls `request_logoff()`.
- The engine manages sending, waiting, and disconnecting.
- This design keeps protocol logic out of the application and ensures robust