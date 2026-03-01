# FIX Session Logic Implementation

This directory contains the implementation of the session logic for the FIX protocol. Below is a detailed explanation of how the code handles various session-related messages and sequence number management.

---

## Logon (35=A)

The logon process is managed by the `LogonHandler` and the `FixEngine.logon()` method:

- The engine creates a logon message using the `fixmsg()` factory.
- If `reset_seq_num_on_logon` is set in the config, the engine will reset both inbound and outbound sequence numbers to 1 and set `ResetSeqNumFlag (141=Y)`.
- The logon message is sent with the correct sender, target, and sequence number.
- The heartbeat mechanism is started after sending Logon.

**Example:**
```python
async def logon(self):
    logon_message = self.fixmsg()
    reset_seq_num_flag_config = self.config_manager.get('FIX', 'reset_seq_num_on_logon', 'false').lower() == 'true'
    if reset_seq_num_flag_config:
        await self.reset_sequence_numbers()
        logon_message[141] = 'Y'
    else:
        logon_message[141] = 'N'
    logon_message.update({35: 'A', 108: self.heartbeat_interval})
    logon_message[98] = int(self.config_manager.get('FIX', 'encryptmethod', '0'))
    await self.send_message(logon_message)
    if self.heartbeat:
        await self.heartbeat.start()
```

---

## Sequence Number Management

Sequence numbers are managed by the `DatabaseMessageStore` class, which provides async methods for storing, retrieving, and updating sequence numbers and messages.

- **Get next incoming/outgoing sequence number:**  
  `get_next_incoming_sequence_number()` and `get_next_outgoing_sequence_number()` (sync)
- **Increment sequence numbers:**  
  `await increment_incoming_sequence_number()` and `await increment_outgoing_sequence_number()` (async)
- **Set/reset sequence numbers:**  
  `await set_incoming_sequence_number(n)`, `await set_outgoing_sequence_number(n)`, `await reset_sequence_numbers()` (async)
- **Store and retrieve messages:**  
  `await store_message(...)`, `await get_message(...)` (async)

**Do not** set sequence numbers directly on the message store from application code.  
**Always use the engine's async API** to ensure protocol compliance and encapsulation.

---

## Message Handling

Handlers for all core FIX session messages are implemented in `message_handler.py` and registered with the `MessageProcessor`.  
All handlers use async methods and the correct await/sync pattern for the message store.

### Example: ResendRequestHandler (35=2)
```python
class ResendRequestHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message):
        start_seq_num = int(message.get(7))
        end_seq_num = int(message.get(16))
        if end_seq_num == 0:
            end_seq_num = self.message_store.get_current_outgoing_sequence_number()
        for seq_num in range(start_seq_num, end_seq_num + 1):
            stored_message_str = await self.message_store.get_message(
                self.engine.version, self.engine.sender, self.engine.target, seq_num
            )
            if stored_message_str:
                # resend logic...
                await self.engine.send_message(...)
            else:
                await self.send_gap_fill(seq_num, seq_num)
```

### Example: SequenceResetHandler (35=4)
```python
class SequenceResetHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message):
        gap_fill_flag = message.get(123) == 'Y'
        new_seq_no = int(message.get(36))
        if not gap_fill_flag:
            await self.message_store.set_incoming_sequence_number(new_seq_no)
            await self.message_store.set_outgoing_sequence_number(new_seq_no)
        else:
            await self.message_store.set_incoming_sequence_number(new_seq_no)
```

---

## Logoff Handshake

The logoff handshake is managed by the engine, not the application.  
Call `await engine.request_logoff()` to initiate a protocol-compliant logoff handshake.  
The engine will send Logout, wait for a response, and disconnect cleanly.

---

## Setting Initial Sequence Numbers

You can set the initial incoming and outgoing sequence numbers for a session in two ways:

### 1. Via `FixEngine` Constructor

```python
engine = await FixEngine.create(
    config_manager,
    application,
    initial_incoming_seqnum=100,
    initial_outgoing_seqnum=200
)
```

### 2. Via Setter Methods (before `engine.start()`)

```python
await engine.set_inbound_sequence_number(100)
await engine.set_outbound_sequence_number(200)
await engine.set_sequence_numbers(incoming_seqnum=100, outgoing_seqnum=200)
```

**Always set sequence numbers before calling `await engine.start()` to ensure correct session state.**

---

## Handling ResetSeqNumFlag (141=Y) and Logon Sequence Numbers

When using `ResetSeqNumFlag=Y` in a Logon message, both initiator and acceptor must reset their sequence numbers to 1.  
After a successful Logon with `ResetSeqNumFlag=Y`, both sides should accept and send Logon messages with `MsgSeqNum=1`.

**Important:**  
If your engine sends Logon with `ResetSeqNumFlag=Y`, it must accept a Logon response from the counterparty with `MsgSeqNum=1`, even if the next expected incoming sequence number would otherwise be higher.  
This ensures interoperability with QuickFIX and other compliant FIX engines.

#### Example (Initiator/Acceptor Logon Exchange):

- Initiator sends: `35=A, 34=1, 141=Y`
- Acceptor responds: `35=A, 34=1, 141=Y`
- Both sides reset their sequence numbers to 1 and proceed with the session.

Your engine's `LogonHandler` is designed to handle this scenario and will accept a Logon response with `MsgSeqNum=1` if `ResetSeqNumFlag=Y` was sent, ensuring protocol compliance.

---

## When to Use Each Method

- **Constructor:** Use for config-driven or one-time setup at engine creation.
- **Setters:** Use for scripting, testing, or when you need to reset sequence numbers dynamically before session start.

---

**Do not** set sequence numbers directly on the message store from application code.  
**Always use the engine's API** to ensure proper encapsulation and protocol compliance.

---

## Additional Notes

- All message store methods that interact with the database and may block are async and must be awaited.
- All sequence number logic in handlers and engine is now fully async-aware and protocol-compliant.
- The engine and handlers are robust to out-of-order, duplicate, and gap-fill scenarios, and will interoperate with QuickFIX and other major FIX engines.
