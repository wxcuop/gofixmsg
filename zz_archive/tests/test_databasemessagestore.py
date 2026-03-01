import pytest
import pytest_asyncio
import os
from pyfixmsg_plus.fixengine.database_message_store import DatabaseMessageStore

print("Loaded DatabaseMessageStore from", __file__)
print("store_message is async:", hasattr(DatabaseMessageStore, "store_message") and callable(getattr(DatabaseMessageStore, "store_message")) and DatabaseMessageStore.store_message.__code__.co_flags & 0x80)

@pytest.fixture
def db_path(tmp_path):
    return os.path.join(tmp_path, "test.db")

@pytest_asyncio.fixture
async def db_store(db_path):
    store = DatabaseMessageStore(db_path)
    store.beginstring = 'FIX.4.4'
    store.sendercompid = 'SENDER'
    store.targetcompid = 'TARGET'
    # Only call initialize if it exists
    await store.initialize()
    return store

@pytest.mark.asyncio
async def test_store_message(db_store):
    await db_store.store_message('FIX.4.4', 'SENDER', 'TARGET', 1, 'Test message')
    result = await db_store.get_message('FIX.4.4', 'SENDER', 'TARGET', 1)
    assert result == 'Test message'

@pytest.mark.asyncio
async def test_sequence_numbers(db_store):
    await db_store.set_incoming_sequence_number(1)
    await db_store.set_outgoing_sequence_number(1)
    incoming_seqnum = db_store.get_next_incoming_sequence_number()
    outgoing_seqnum = db_store.get_next_outgoing_sequence_number()
    assert incoming_seqnum == 1
    assert outgoing_seqnum == 1

    # Test increment
    await db_store.increment_incoming_sequence_number()
    await db_store.increment_outgoing_sequence_number()
    assert db_store.get_next_incoming_sequence_number() == 2
    assert db_store.get_next_outgoing_sequence_number() == 2

    await db_store.reset_sequence_numbers()
    await db_store.set_incoming_sequence_number(1)  # Reset to 1 for the test
    await db_store.set_outgoing_sequence_number(1)  # Reset to 1 for the test
    assert db_store.get_next_incoming_sequence_number() == 1
    assert db_store.get_next_outgoing_sequence_number() == 1
