import sqlite3
from datetime import datetime, UTC
import logging
import asyncio
import os

class DatabaseMessageStore:
    def __init__(self, db_path, beginstring=None, sendercompid=None, targetcompid=None):
        self.db_path = db_path
        # Ensure parent directory exists
        db_dir = os.path.dirname(db_path)
        if db_dir and not os.path.exists(db_dir):
            os.makedirs(db_dir, exist_ok=True)
        self.conn = sqlite3.connect(db_path)
        self.logger = logging.getLogger(self.__class__.__name__)
        self.create_table()
        self._lock = asyncio.Lock()
        self.beginstring = beginstring
        self.sendercompid = sendercompid
        self.targetcompid = targetcompid
        self.incoming_seqnum = 1
        self.outgoing_seqnum = 1

    async def initialize(self):
        if self.beginstring and self.sendercompid and self.targetcompid:
            loaded_in, loaded_out = await self.load_sequence_numbers()
            self.incoming_seqnum = loaded_in
            self.outgoing_seqnum = loaded_out
            self.logger.info(f"Loaded sequence numbers for session {self.beginstring}-{self.sendercompid}-{self.targetcompid}: NextIncoming={self.incoming_seqnum}, NextOutgoing={self.outgoing_seqnum}")
        else:
            self.logger.info("Session identifiers not set at init. Defaulting sequence numbers to 1 (as next expected).")

    def create_table(self):
        cursor = self.conn.cursor()
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS messages (
                beginstring TEXT NOT NULL,
                sendercompid TEXT NOT NULL, 
                targetcompid TEXT NOT NULL, 
                msgseqnum INTEGER NOT NULL,
                message TEXT NOT NULL,
                timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (beginstring, sendercompid, targetcompid, msgseqnum) 
            )
        ''')
        cursor.execute('''
            CREATE TABLE IF NOT EXISTS sessions (
                beginstring TEXT NOT NULL,
                sendercompid TEXT NOT NULL,
                targetcompid TEXT NOT NULL,
                creation_time TEXT NOT NULL,
                next_incoming_seqnum INTEGER NOT NULL, 
                next_outgoing_seqnum INTEGER NOT NULL, 
                PRIMARY KEY (beginstring, sendercompid, targetcompid)
            )
        ''')
        self.conn.commit()

    async def store_message(self, beginstring, sendercompid, targetcompid, msgseqnum, message):
        async with self._lock:
            try:
                cursor = self.conn.cursor()
                cursor.execute('''
                    INSERT OR REPLACE INTO messages (beginstring, sendercompid, targetcompid, msgseqnum, message)
                    VALUES (?, ?, ?, ?, ?)
                ''', (beginstring, sendercompid, targetcompid, msgseqnum, message))
                self.conn.commit()
                self.logger.debug(f"Stored message: {sendercompid}->{targetcompid} Seq={msgseqnum}")
            except Exception as e:
                self.logger.error(f"Error storing message for Seq={msgseqnum}: {e}", exc_info=True)

    async def get_message(self, beginstring, sendercompid, targetcompid, msgseqnum):
        async with self._lock:
            try:
                cursor = self.conn.cursor()
                cursor.execute('''
                    SELECT message FROM messages WHERE beginstring = ? AND sendercompid = ? AND targetcompid = ? AND msgseqnum = ?
                ''', (beginstring, sendercompid, targetcompid, msgseqnum))
                result = cursor.fetchone()
                if result:
                    self.logger.debug(f"Retrieved message for {sendercompid}->{targetcompid} Seq={msgseqnum}")
                    return result[0]
                else:
                    self.logger.debug(f"No message found for {sendercompid}->{targetcompid} Seq={msgseqnum}")
                    return None
            except Exception as e:
                self.logger.error(f"Error retrieving message for Seq={msgseqnum}: {e}", exc_info=True)
                return None

    async def load_sequence_numbers(self):
        if self.beginstring and self.sendercompid and self.targetcompid:
            try:
                cursor = self.conn.cursor()
                cursor.execute('''
                    SELECT next_incoming_seqnum, next_outgoing_seqnum FROM sessions WHERE
                    beginstring = ? AND sendercompid = ? AND targetcompid = ?
                ''', (self.beginstring, self.sendercompid, self.targetcompid))
                result = cursor.fetchone()
                if result:
                    self.logger.debug(f"Loaded sequence numbers from DB: NextIncoming={result[0]}, NextOutgoing={result[1]}")
                    return int(result[0]), int(result[1]) 
            except Exception as e:
                self.logger.error(f"Error loading sequence numbers: {e}", exc_info=True)
        self.logger.debug("No sequence numbers found in DB for session, defaulting to 1,1 (next expected).")
        return 1, 1 

    async def save_sequence_numbers(self):
        if not (self.beginstring and self.sendercompid and self.targetcompid):
            self.logger.warning("Cannot save sequence numbers: session identifiers not set.")
            return
        try:
            cursor = self.conn.cursor()
            cursor.execute('''
                INSERT INTO sessions (beginstring, sendercompid, targetcompid, creation_time, next_incoming_seqnum, next_outgoing_seqnum)
                VALUES (?, ?, ?, ?, ?, ?)
                ON CONFLICT(beginstring, sendercompid, targetcompid)
                DO UPDATE SET next_incoming_seqnum = excluded.next_incoming_seqnum, 
                              next_outgoing_seqnum = excluded.next_outgoing_seqnum,
                              creation_time = CASE WHEN excluded.next_incoming_seqnum = 1 AND excluded.next_outgoing_seqnum = 1 
                                                   THEN excluded.creation_time 
                                                   ELSE creation_time END
            ''', (self.beginstring, self.sendercompid, self.targetcompid, 
                  datetime.now(UTC).strftime("%Y-%m-%d %H:%M:%S"),
                  self.incoming_seqnum, self.outgoing_seqnum))
            self.conn.commit()
            self.logger.debug(f"Saved sequence numbers: Next Incoming={self.incoming_seqnum}, Next Outgoing={self.outgoing_seqnum}")
        except Exception as e:
            self.logger.error(f"Error saving sequence numbers: {e}", exc_info=True)

    def get_next_incoming_sequence_number(self) -> int:
        return self.incoming_seqnum

    async def increment_incoming_sequence_number(self):
        async with self._lock:
            self.incoming_seqnum += 1
            await self.save_sequence_numbers()
            self.logger.debug(f"Incremented incoming sequence. Next expected is now: {self.incoming_seqnum}")

    def get_next_outgoing_sequence_number(self) -> int:
        return self.outgoing_seqnum

    async def increment_outgoing_sequence_number(self):
        async with self._lock:
            self.outgoing_seqnum += 1
            await self.save_sequence_numbers()
            self.logger.debug(f"Incremented outgoing sequence. Next to be used is now: {self.outgoing_seqnum}")

    async def set_incoming_sequence_number(self, number: int):
        if not isinstance(number, int) or number < 1:
            self.logger.error(f"Invalid attempt to set incoming sequence number to: {number}")
            return
        async with self._lock:
            self.incoming_seqnum = number
            await self.save_sequence_numbers()
            self.logger.info(f"Next incoming sequence number set to: {self.incoming_seqnum}")

    async def set_outgoing_sequence_number(self, number: int):
        if not isinstance(number, int) or number < 1:
            self.logger.error(f"Invalid attempt to set outgoing sequence number to: {number}")
            return
        async with self._lock:
            self.outgoing_seqnum = number
            await self.save_sequence_numbers()
            self.logger.info(f"Next outgoing sequence number set to: {self.outgoing_seqnum}")

    async def reset_sequence_numbers(self):
        self.logger.info(f"Resetting sequence numbers for session {self.beginstring}-{self.sendercompid}-{self.targetcompid} to 1.")
        async with self._lock:
            self.incoming_seqnum = 1
            self.outgoing_seqnum = 1
            await self.save_sequence_numbers()

    def is_new_session(self) -> bool:
        is_new = (self.incoming_seqnum == 1 and self.outgoing_seqnum == 1)
        self.logger.debug(f"is_new_session check: NextIncoming={self.incoming_seqnum}, NextOutgoing={self.outgoing_seqnum}. Is new? {is_new}")
        return is_new

    def get_current_outgoing_sequence_number(self) -> int:
        if self.outgoing_seqnum > 1:
            last_used = self.outgoing_seqnum - 1
            self.logger.debug(f"get_current_outgoing_sequence_number: NextOutgoing is {self.outgoing_seqnum}, so current (last used) is {last_used}")
            return last_used
        else:
            self.logger.debug("get_current_outgoing_sequence_number: NextOutgoing is 1, so no messages sent yet in this context. Returning 0.")
            return 0

    async def shutdown(self):
        """
        Wait for all pending DB operations to finish before closing the DB.
        Call this before close() during shutdown.
        """
        async with self._lock:
            pass  # Ensures all other operations using the lock are done

    async def close(self):
        """
        Async close: ensures all pending DB operations are finished before closing the DB.
        Always call this with 'await'.
        """
        await self.shutdown()
        if self.conn:
            self.conn.close()
            self.conn = None
            self.logger.info("Database connection closed.")

# Example usage (updated to reflect new get_next/increment pattern)
if __name__ == "__main__":
    import os
    db_file = 'fix_messages_test.db'
    if os.path.exists(db_file):
        os.remove(db_file)
    
    logging.basicConfig(level=logging.DEBUG, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')

    async def main():
        print("\n--- Test Case 1: New Session ---")
        store1 = DatabaseMessageStore(db_file, 'FIX.4.4', 'SENDER1', 'TARGET1')
        await store1.initialize()
        print(f"Initial: NextIn={store1.get_next_incoming_sequence_number()}, NextOut={store1.get_next_outgoing_sequence_number()}, IsNew={store1.is_new_session()}")
        
        seq_to_send1 = store1.get_next_outgoing_sequence_number() # Should be 1
        print(f"Seq to send1: {seq_to_send1}")
        await store1.store_message('FIX.4.4', 'SENDER1', 'TARGET1', seq_to_send1, f"Message {seq_to_send1} from SENDER1")
        await store1.increment_outgoing_sequence_number() # Increment after using 1, internal next becomes 2
        print(f"After send 1: NextIn={store1.get_next_incoming_sequence_number()}, NextOut={store1.get_next_outgoing_sequence_number()}, CurrentOut={store1.get_current_outgoing_sequence_number()}")
        
        print(f"GetNextIn (doesn't increment): {store1.get_next_incoming_sequence_number()}") # Should be 1
        await store1.increment_incoming_sequence_number() # Simulate processing incoming message 1, internal next becomes 2
        print(f"After receive 1: NextIn={store1.get_next_incoming_sequence_number()}, NextOut={store1.get_next_outgoing_sequence_number()}")
        store1.close()

        print("\n--- Test Case 2: Load Existing Session ---")
        store2 = DatabaseMessageStore(db_file, 'FIX.4.4', 'SENDER1', 'TARGET1')
        await store2.initialize()
        print(f"Loaded: NextIn={store2.get_next_incoming_sequence_number()}, NextOut={store2.get_next_outgoing_sequence_number()}, IsNew={store2.is_new_session()}")
        
        seq_to_send2 = store2.get_next_outgoing_sequence_number() # Should be 2
        print(f"Seq to send2: {seq_to_send2}")
        await store2.store_message('FIX.4.4', 'SENDER1', 'TARGET1', seq_to_send2, f"Message {seq_to_send2} from SENDER1")
        await store2.increment_outgoing_sequence_number() # Increment after using 2, internal next becomes 3
        print(f"After send 2: NextOut={store2.get_next_outgoing_sequence_number()}, CurrentOut={store2.get_current_outgoing_sequence_number()}") # Should be 3, 2
        store2.close()

        print("\n--- Test Case 3: Reset ---")
        store3 = DatabaseMessageStore(db_file, 'FIX.4.4', 'SENDER1', 'TARGET1')
        await store3.initialize()
        print(f"Before Reset: NextIn={store3.get_next_incoming_sequence_number()}, NextOut={store3.get_next_outgoing_sequence_number()}")
        await store3.reset_sequence_numbers()
        print(f"After Reset: NextIn={store3.get_next_incoming_sequence_number()}, NextOut={store3.get_next_outgoing_sequence_number()}, IsNew={store3.is_new_session()}")
        print(f"CurrentOut after reset: {store3.get_current_outgoing_sequence_number()}") # Should be 0
        store3.close()

        print("\n--- Test Case 4: Set Identifiers Later ---")
        if os.path.exists(db_file): os.remove(db_file) # Clean slate
        store4_no_id = DatabaseMessageStore(db_file)
        print(f"No ID: NextIn={store4_no_id.get_next_incoming_sequence_number()}, NextOut={store4_no_id.get_next_outgoing_sequence_number()}")
        # You may want to make set_session_identifiers async if it loads from DB
        # await store4_no_id.set_session_identifiers('FIX.4.2', 'SENDERX', 'TARGETX')
        store4_no_id.set_session_identifiers('FIX.4.2', 'SENDERX', 'TARGETX')
        print(f"ID Set: NextIn={store4_no_id.get_next_incoming_sequence_number()}, NextOut={store4_no_id.get_next_outgoing_sequence_number()}, IsNew={store4_no_id.is_new_session()}")
        
        first_out_s4 = store4_no_id.get_next_outgoing_sequence_number()
        await store4_no_id.store_message('FIX.4.2', 'SENDERX', 'TARGETX', first_out_s4, "Test S4")
        await store4_no_id.increment_outgoing_sequence_number()
        print(f"CurrentOut S4: {store4_no_id.get_current_outgoing_sequence_number()}")
        store4_no_id.close()

    asyncio.run(main())
