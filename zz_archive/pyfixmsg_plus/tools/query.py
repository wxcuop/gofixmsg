import argparse
import asyncio
from pyfixmsg_plus.fixengine import FixEngine, ConfigManager

def print_message(msg):
    if msg is None:
        print("No message found.")
    else:
        for tag, value in msg.items():
            print(f"{tag}: {value}")

async def main():
    parser = argparse.ArgumentParser(
        description="Query FIX messages via the engine's API."
    )
    parser.add_argument(
        "--config", required=True, help="Path to the FIX engine config file (e.g., config_initiator.ini)"
    )
    parser.add_argument(
        "--session", required=True, help="Session ID in the form SENDER-TARGET (e.g., BANZAI-EXEC)"
    )
    parser.add_argument(
        "--seqnum", type=int, help="Sequence number to query (if omitted, shows all for session)"
    )
    parser.add_argument(
        "--clordid", help="Query by ClOrdID (tag 11) value"
    )

    args = parser.parse_args()

    sender, target = args.session.split("-", 1)
    config = ConfigManager(args.config)
    # Use async engine creation to ensure message_store is initialized
    engine = await FixEngine.create(config, application=None)
    store = engine.message_store

    if args.clordid:
        # Query by ClOrdID (tag 11)
        cursor = store.conn.cursor()
        cursor.execute(
            "SELECT msgseqnum, message, timestamp FROM messages WHERE sendercompid=? AND targetcompid=? ORDER BY msgseqnum",
            (sender, target),
        )
        rows = cursor.fetchall()
        found = False
        for row in rows:
            msgseqnum, message, ts = row
            from pyfixmsg.fixmessage import FixMessage
            fixmsg = FixMessage()
            try:
                fixmsg.load_fix(message)
            except Exception:
                continue
            if 11 in fixmsg and str(fixmsg[11]) == args.clordid:
                print(f"SeqNum: {msgseqnum}, Timestamp: {ts}")
                print(message)
                print("-" * 40)
                found = True
        if not found:
            print(f"No message found with ClOrdID (tag 11) = {args.clordid}")
        return

    if args.seqnum:
        # Query current (get_message is async)
        msg = await store.get_message(store.beginstring, sender, target, args.seqnum)
        print_message(msg)
    else:
        # Show all messages for session
        cursor = store.conn.cursor()
        cursor.execute(
            "SELECT msgseqnum, message, timestamp FROM messages WHERE sendercompid=? AND targetcompid=? ORDER BY msgseqnum",
            (sender, target),
        )
        rows = cursor.fetchall()
        for row in rows:
            print(f"SeqNum: {row[0]}, Timestamp: {row[2]}")
            print(row[1])
            print("-" * 40)

if __name__ == "__main__":
    asyncio.run(main())