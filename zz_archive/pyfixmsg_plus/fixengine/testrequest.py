from datetime import datetime, timezone

class TestRequest:
    def __init__(self, send_message_callback, config_manager, fix_message_creator): # Added fix_message_creator
        self.send_message = send_message_callback
        self.config_manager = config_manager
        self.fix_message_creator = fix_message_creator # Store the creator

    async def send_test_request(self):
        # Use datetime.now(timezone.utc) instead of deprecated utcnow()
        test_req_id = datetime.now(timezone.utc).strftime("%Y%m%d-%H:%M:%S.%f")[:-3] # Ensure milliseconds
        
        fields = {
            35: '1',  # MsgType: TestRequest
            49: self.config_manager.get('FIX', 'sender', 'SENDER'), # Use configured sender
            56: self.config_manager.get('FIX', 'target', 'TARGET'), # Use configured target
            112: test_req_id  # TestReqID
        }
        # Use the provided fix_message_creator (engine.fixmsg)
        message = self.fix_message_creator(fields) # Create a new message instance
        # Other header fields like SendingTime, MsgSeqNum will be added by engine.send_message
        await self.send_message(message)
        return test_req_id

# Example usage (updated to reflect new __init__ but won't run standalone without mocks)
if __name__ == "__main__":
    import asyncio
    import logging # Added for context

    # Mock a config manager
    class MockConfigManager:
        def get(self, section, key, default=None):
            if key == 'sender': return 'TESTSENDER'
            if key == 'target': return 'TESTTARGET'
            return default

    # Mock a fix_message_creator (like engine.fixmsg)
    class MockFixMessage:
        def __init__(self, initial_data=None):
            self._tags = initial_data if initial_data else {}
            self.codec = None # Mock codec attribute
        def update(self, data_dict):
            self._tags.update(data_dict)
        def get(self, tag):
            return self._tags.get(tag)
        def __setitem__(self, key, value):
            self._tags[key] = value
        def __getitem__(self, key):
            return self._tags[key]
        def __contains__(self, key):
            return key in self._tags
        def to_wire(self, codec=None, pretty=False): # Mock method
            return str(self._tags)


    def mock_fix_msg_creator(initial_data=None):
        return MockFixMessage(initial_data)

    async def send_message_cb(message):
        logging.info(f"Mock Sending message: {message.to_wire()}")

    logging.basicConfig(level=logging.INFO)
    config_mgr = MockConfigManager()
    
    # Instantiate TestRequest with the mock creator
    test_request_sender = TestRequest(send_message_cb, config_mgr, mock_fix_msg_creator)
    
    async def main():
        req_id = await test_request_sender.send_test_request()
        logging.info(f"Sent TestRequest with ID: {req_id}")

    asyncio.run(main())
