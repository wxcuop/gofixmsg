import asyncio
import logging

class GapFill:
    def __init__(self, message_store):
        self.message_store = message_store
        self.logger = logging.getLogger('GapFill')

    async def handle_gap_fill(self, message):
        new_seq_no = int(message.get('36'))  # NewSeqNo
        gap_fill_message = {
            '35': '4',  # Sequence Reset
            '123': 'Y',  # GapFillFlag
            '36': new_seq_no  # NewSeqNo
        }
        await self.message_store.store_message(gap_fill_message)
        self.logger.info(f"Handled Gap Fill to NewSeqNo {new_seq_no}")

# Example usage
if __name__ == "__main__":
    class DummyMessageStore:
        async def store_message(self, message):
            print(f"Storing message: {message}")

    message_store = DummyMessageStore()
    gap_fill = GapFill(message_store)
    asyncio.run(gap_fill.handle_gap_fill({'36': '100'}))  # Example gap fill message with NewSeqNo of 100
