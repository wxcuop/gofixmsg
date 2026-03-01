# pyfixmsg_plus.tools.query — FIX Message Query Tool

This command-line tool allows you to query FIX messages from your session database using the engine's API.  
You can filter by session, sequence number, or ClOrdID (tag 11).

---

## Usage

```sh
python -m pyfixmsg_plus.tools.query --config <config_file> --session <SENDER-TARGET> [options]
```

### Required Arguments

- `--config <config_file>`  
  Path to your FIX engine config file (e.g., `examples/config_initiator.ini`).

- `--session <SENDER-TARGET>`  
  Session identifier in the form `SENDER-TARGET` (e.g., `BANZAI-EXEC`).

### Optional Arguments

- `--seqnum <N>`  
  Query a specific sequence number.

- `--clordid <ClOrdID>`  
  Query messages by ClOrdID (tag 11) value.

---

## Examples

### Show all current messages for a session

```sh
python -m pyfixmsg_plus.tools.query --config examples/config_initiator.ini --session BANZAI-EXEC
```

### Show a specific message by sequence number

```sh
python -m pyfixmsg_plus.tools.query --config examples/config_initiator.ini --session BANZAI-EXEC --seqnum 42
```

### Query by ClOrdID (tag 11)

```sh
python -m pyfixmsg_plus.tools.query --config examples/config_initiator.ini --session BANZAI-EXEC --clordid TestOrd-123456
```

---

## Notes

- If you use `--clordid`, the tool will search all messages for the given ClOrdID value.
- If you omit `--seqnum` and `--clordid`, all messages for the session will be listed.
- The tool prints the raw FIX message and basic metadata for each result.
