from pyfixmsg_plus.fixengine.database_message_store import DatabaseMessageStore
from pyfixmsg_plus.fixengine.database_message_store_aiosqlite import DatabaseMessageStoreAioSqlite
from typing import Optional, Any

try:
    from pyfixmsg_plus.fixengine.database_message_store_aiosqlite import DatabaseMessageStoreAioSqlite
except ImportError:
    DatabaseMessageStoreAioSqlite = None

class MessageStoreFactory:
    @staticmethod
    async def get_message_store(
        store_type: str,
        db_path: str,
        beginstring: Optional[str] = None,
        sendercompid: Optional[str] = None,
        targetcompid: Optional[str] = None
    ) -> Any:
        if store_type == 'database':
            store = DatabaseMessageStore(db_path, beginstring, sendercompid, targetcompid)
            await store.initialize()
            return store
        elif store_type == 'aiosqlite':
            if DatabaseMessageStoreAioSqlite is None:
                raise ImportError("The 'aiosqlite' store type requires the aiosqlite library, which is not installed.")
            store = DatabaseMessageStoreAioSqlite(db_path, beginstring, sendercompid, targetcompid)
            await store.initialize()
            return store
        else:
            raise ValueError(f"Unknown store type: {store_type}")
