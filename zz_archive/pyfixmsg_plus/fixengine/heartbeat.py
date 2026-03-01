import asyncio
import logging
from pyfixmsg_plus.fixengine.testrequest import TestRequest # Assuming TestRequest is in the same directory or path

class Heartbeat:
    def __init__(self, send_message_callback, config_manager, interval, state_machine, fix_engine): # fix_engine is available
        self.send_message_callback = send_message_callback
        self.config_manager = config_manager
        self.interval = interval
        self.state_machine = state_machine
        self.fix_engine = fix_engine # Store fix_engine
        self.logger = logging.getLogger(self.__class__.__name__)
        self.heartbeat_task = None
        self.test_request_task = None
        self.last_sent_time = 0
        self.last_received_time = 0
        self.missed_heartbeats = 0
        self.test_request_id = None # Store the ID of the last sent TestRequest

        # Instantiate TestRequest correctly, passing the engine's fixmsg method
        if self.fix_engine and hasattr(self.fix_engine, 'fixmsg'):
            self.test_request_sender = TestRequest(
                self.send_message_callback, 
                self.config_manager, 
                self.fix_engine.fixmsg  # Pass the engine's message factory
            )
        else:
            self.logger.error("FixEngine or fix_engine.fixmsg not available to Heartbeat for TestRequest instantiation.")
            self.test_request_sender = None


    async def start(self):
        if not self.is_running():
            self.last_sent_time = asyncio.get_event_loop().time()
            self.last_received_time = asyncio.get_event_loop().time() # Initialize on start
            self.missed_heartbeats = 0
            self.heartbeat_task = asyncio.create_task(self._run_heartbeat_logic())
            self.logger.info("Heartbeat mechanism started.")
        else:
            self.logger.info("Heartbeat mechanism already running.")

    async def stop(self):
        if self.is_running():
            self.heartbeat_task.cancel()
            try:
                await self.heartbeat_task
            except asyncio.CancelledError:
                self.logger.info("Heartbeat task cancelled.")
            finally:
                self.heartbeat_task = None
        if self.test_request_task and not self.test_request_task.done():
            self.test_request_task.cancel()
            try:
                await self.test_request_task
            except asyncio.CancelledError:
                self.logger.info("TestRequest task cancelled during heartbeat stop.")
            finally:
                self.test_request_task = None
        self.logger.info("Heartbeat mechanism stopped.")

    def is_running(self):
        return self.heartbeat_task is not None and not self.heartbeat_task.done()

    async def _run_heartbeat_logic(self):
        try:
            while True:
                if self.state_machine.state.name != 'ACTIVE':
                    self.logger.debug("Session not ACTIVE. Heartbeat logic paused.")
                    await asyncio.sleep(self.interval / 2) # Check state more frequently when not active
                    continue

                now = asyncio.get_event_loop().time()
                
                # Check for sending heartbeat
                if (now - self.last_sent_time) >= self.interval:
                    hb_msg = self.fix_engine.fixmsg({35: '0'}) # Use engine.fixmsg
                    if self.test_request_id: # If we are expecting a reply to a TestRequest
                        hb_msg[112] = self.test_request_id # Include TestReqID in outgoing Heartbeat
                        self.logger.info(f"Sending Heartbeat with TestReqID {self.test_request_id} (still awaiting response).")
                    else:
                        self.logger.debug("Sending regular Heartbeat.")
                    await self.send_message_callback(hb_msg)
                    self.last_sent_time = now

                # Check for receiving heartbeat
                # Max interval factor: e.g., 2.5 times the interval. Some use 1.2, some 1.5, some 2+.
                # Let's use a factor slightly larger than 1, e.g., 1.2 or 1.5, for sending TestRequest
                # And a larger factor for timeout, e.g., 2 to 2.5
                test_request_threshold = self.interval * 1.2 # Threshold to send TestRequest
                timeout_threshold = self.interval * 2.5    # Threshold to disconnect

                if (now - self.last_received_time) > timeout_threshold:
                    self.logger.error(f"No message received for too long (>{timeout_threshold:.2f}s). Disconnecting session.")
                    await self.fix_engine.disconnect(graceful=False) # Access disconnect via fix_engine
                    break # Exit heartbeat loop

                if not self.test_request_id and (now - self.last_received_time) > test_request_threshold:
                    if self.test_request_sender:
                        self.logger.warning(f"No message received for >{test_request_threshold:.2f}s. Sending TestRequest.")
                        self.test_request_id = await self.test_request_sender.send_test_request() # Store the ID
                        self.logger.info(f"Sent TestRequest, ID: {self.test_request_id}")
                        # After sending a TestRequest, we give some time for the Heartbeat response.
                        # The check for timeout_threshold above will handle if no response comes at all.
                    else:
                        self.logger.error("Cannot send TestRequest: test_request_sender not initialized.")
                
                await asyncio.sleep(1)  # Check every second
        except asyncio.CancelledError:
            self.logger.info("Heartbeat logic task was cancelled.")
        except Exception as e:
            self.logger.error(f"Error in heartbeat logic: {e}", exc_info=True)
            if self.fix_engine: # Attempt to disconnect if engine is available
                await self.fix_engine.disconnect(graceful=False)

    def set_remote_interval(self, remote_interval_seconds):
        # This method might be called by LogonHandler if the counterparty specifies a HeartBtInt.
        # The FIX standard suggests using the *sender's* HeartBtInt for sending heartbeats,
        # and the *receiver's* HeartBtInt (from Logon) for monitoring incoming heartbeats.
        # For simplicity, many engines use their own configured interval for sending,
        # and use the received interval for monitoring.
        # Here, self.interval is our sending interval. We can log the remote one.
        self.logger.info(f"Counterparty HeartBtInt: {remote_interval_seconds}s. We will send every {self.interval}s.")
        # If you need to adjust monitoring based on remote_interval, do it here.
        # For now, our monitoring (timeout_threshold, test_request_threshold) is based on our self.interval.

    def process_incoming_heartbeat(self, test_req_id_in_hb=None):
        """Called by HeartbeatHandler when a Heartbeat is received."""
        now = asyncio.get_event_loop().time()
        self.logger.debug(f"Processing incoming Heartbeat. Current time: {now}, Last received: {self.last_received_time}")
        self.last_received_time = now
        self.missed_heartbeats = 0 # Reset counter
        
        if test_req_id_in_hb:
            self.logger.debug(f"Incoming Heartbeat contains TestReqID: {test_req_id_in_hb}. Our pending TestReqID: {self.test_request_id}")
            if self.test_request_id and test_req_id_in_hb == self.test_request_id:
                self.logger.info(f"Heartbeat with TestReqID {test_req_id_in_hb} matches our pending TestRequest. Clearing pending TestRequest.")
                self.test_request_id = None # Clear our pending TestRequest ID as it's been satisfied
            elif self.test_request_id:
                 self.logger.warning(f"Heartbeat TestReqID {test_req_id_in_hb} does not match our pending {self.test_request_id}.")
            # else: no pending test_request_id from our side.
        else: # Regular heartbeat, not in response to a TestRequest we sent
            if self.test_request_id:
                self.logger.debug(f"Received regular Heartbeat, but still awaiting response for TestReqID: {self.test_request_id}")
            # else: regular heartbeat, no pending test request from our side.
