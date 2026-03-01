import asyncio
import inspect
import logging
from datetime import datetime, timezone
from pyfixmsg_plus.fixengine.heartbeat_builder import HeartbeatBuilder
from pyfixmsg_plus.fixengine.testrequest import TestRequest 
from pyfixmsg_plus.fixengine.network import Acceptor, Initiator
from pyfixmsg_plus.fixengine.configmanager import ConfigManager
from pyfixmsg_plus.fixengine.message_handler import (
    MessageProcessor,
    LogonHandler,
    TestRequestHandler,
    ExecutionReportHandler,
    NewOrderHandler,
    CancelOrderHandler,
    OrderCancelReplaceHandler,
    OrderCancelRejectHandler,
    NewOrderMultilegHandler,
    MultilegOrderCancelReplaceHandler,
    ResendRequestHandler,
    SequenceResetHandler,
    RejectHandler,
    LogoutHandler,
    HeartbeatHandler
)
from pyfixmsg_plus.fixengine.message_store_factory import MessageStoreFactory
from pyfixmsg_plus.fixengine.state_machine import (
    StateMachine, Disconnected)
from pyfixmsg_plus.fixengine.scheduler import Scheduler
from pyfixmsg.fixmessage import FixMessage, FixFragment
from pyfixmsg.reference import FixSpec
from pyfixmsg.codecs.stringfix import Codec
from typing import Optional,Any


class FixEngine:
    def __init__(
        self,
        config_manager: ConfigManager,
        application: Any,
        initial_incoming_seqnum: Optional[int] = None,
        initial_outgoing_seqnum: Optional[int] = None
    ):
        self.config_manager = config_manager
        self.application = application
        self.state_machine = StateMachine(Disconnected()) 
        self.state_machine.subscribe(self.on_state_change)
        
        self.host = self.config_manager.get('FIX', 'host', '127.0.0.1')
        self.port = int(self.config_manager.get('FIX', 'port', '5000'))
        self.sender = self.config_manager.get('FIX', 'sender', 'SENDER')
        self.target = self.config_manager.get('FIX', 'target', 'TARGET')
        self.version = self.config_manager.get('FIX', 'version', 'FIX.4.4')
        self.spec_filename = self.config_manager.get('FIX', 'spec_filename', 'FIX44.xml')
        self.use_tls = self.config_manager.get('FIX', 'use_tls', 'false').lower() == 'true'
        self.mode = self.config_manager.get('FIX', 'mode', 'initiator').lower()

        self.logger = logging.getLogger('FixEngine')

        db_path = self.config_manager.get('FIX', 'state_file', 'fix_state.db')
        self.message_store = None

        self._initial_incoming_seqnum = initial_incoming_seqnum
        self._initial_outgoing_seqnum = initial_outgoing_seqnum

        self.heartbeat_interval = int(self.config_manager.get('FIX', 'heartbeat_interval', '30'))
        self.lock = asyncio.Lock()
        
        self.test_request = TestRequest(self.send_message, self.config_manager, self.fixmsg)

        self.heartbeat = (HeartbeatBuilder()
                          .set_send_message_callback(self.send_message)
                          .set_config_manager(self.config_manager)
                          .set_heartbeat_interval(self.heartbeat_interval)
                          .set_state_machine(self.state_machine)
                          .set_fix_engine(self)
                          .build())
        
        self.session_id = f"{self.sender}-{self.target}-{self.host}:{self.port}"
        self.network = Acceptor(self.host, self.port, self.use_tls) if self.mode == 'acceptor' else Initiator(self.host, self.port, self.use_tls)

        self.message_processor = MessageProcessor(self.message_store, self.state_machine, self.application, self)

        handler_classes = {
            'A': LogonHandler,
            '1': TestRequestHandler,
            '8': ExecutionReportHandler,
            'D': NewOrderHandler,
            'F': CancelOrderHandler,
            'G': OrderCancelReplaceHandler,
            '9': OrderCancelRejectHandler,
            'AB': NewOrderMultilegHandler,
            'AC': MultilegOrderCancelReplaceHandler,
            '2': ResendRequestHandler,
            '4': SequenceResetHandler,
            '3': RejectHandler,
            '5': LogoutHandler,
            '0': HeartbeatHandler,
        }
        for msg_type, handler_cls in handler_classes.items():
            self.message_processor.register_handler(
                msg_type, handler_cls(self.message_store, self.state_machine, self.application, self)
            )

        self.scheduler = Scheduler(self.config_manager, self)
        self.scheduler_task = None

        self.retry_interval = int(self.config_manager.get('FIX', 'retry_interval', '5'))
        self.max_retries = int(self.config_manager.get('FIX', 'max_retries', '5'))
        self.retry_attempts = 0

        self.spec = FixSpec(self.spec_filename)
        self.codec = Codec(spec=self.spec, fragment_class=FixFragment)
        self.incoming_buffer = b"" 

        self.resend_request_outstanding = False
        self.resend_request_expected_seq = None

    def fixmsg(self, fields: dict) -> FixMessage:
        message = FixMessage(fields)
        message.codec = self.codec
        message[8] = self.version
        if 52 not in fields:
            fields[52] = datetime.now(timezone.utc).strftime('%Y%m%d-%H:%M:%S.%f')[:-3]
        return message

    def create_message_with_repeating_group(
        self, msg_type: str, group_data: dict, **kwargs
    ) -> FixMessage:
        initial_fields = {35: msg_type}
        initial_fields.update(kwargs)
        message = self.fixmsg(initial_fields)
        for num_in_group_tag, group_entries_data in group_data.items():
            if not isinstance(group_entries_data, list):
                self.logger.error(f"Data for group {num_in_group_tag} must be a list of dictionaries.")
                continue
            fragments_list = []
            for entry_data in group_entries_data:
                if not isinstance(entry_data, dict):
                    self.logger.error(f"Entry in group {num_in_group_tag} must be a dictionary. Got: {type(entry_data)}")
                    continue
                fragment = FixFragment(entry_data)
                fragments_list.append(fragment)
            message[num_in_group_tag] = fragments_list
        return message

    def on_state_change(self, state_name: str) -> None:
        self.logger.info(f"STATE CHANGE ({self.session_id}): {state_name}")
        if state_name == "ACTIVE": 
            self.logger.info(f"Session {self.session_id} is now ACTIVE.")
            self.retry_attempts = 0
        elif state_name == "DISCONNECTED": 
            self.logger.info(f"Session {self.session_id} is DISCONNECTED.")
            if self.heartbeat and self.heartbeat.is_running():
                 self.logger.debug("Stopping heartbeat task due to disconnect.")
                 asyncio.create_task(self.heartbeat.stop())
            self.incoming_buffer = b"" 

    async def start(self) -> None:
        if not self.scheduler_task or self.scheduler_task.done():
            self.scheduler_task = asyncio.create_task(self.scheduler.run_scheduler())
            self.logger.info("Scheduler task started.")
        self.incoming_buffer = b"" 

        try:
            if self.mode == 'acceptor':
                self.logger.info(f"Acceptor starting on {self.host}:{self.port} for session {self.session_id}...")
                await self.network.start_accepting(self.handle_incoming_connection)
                self.logger.info(f"Acceptor {self.session_id} has stopped listening.")
            else: 
                self.logger.info(f"Initiator {self.session_id} starting to connect to {self.host}:{self.port}...")
                self.state_machine.on_event('initiator_connect_attempt') 
                await self.network.connect() 
                self.logger.info(f"Initiator {self.session_id} TCP connected. Proceeding with FIX Logon.")
                self.state_machine.on_event('connection_established') 
                await self.logon()
                await self.receive_message()
        except ConnectionRefusedError as e:
            self.logger.error(f"Connection refused for {self.session_id}: {e}")
            if self.mode == 'initiator':
                self.state_machine.on_event('connection_failed') 
                await self.retry_connect()
        except Exception as e:
            self.logger.error(f"Failed to start or run FIX engine {self.session_id}: {e}", exc_info=True)
            if self.mode == 'initiator':
                self.state_machine.on_event('connection_failed') 
                await self.retry_connect()
        finally:
            self.logger.info(f"FIX Engine {self.session_id} start/run attempt concluded.")

    async def retry_connect(self) -> None:
        if self.mode == 'acceptor': return

        if self.state_machine.state.name != "ACTIVE" and self.retry_attempts < self.max_retries:
            self.retry_attempts += 1
            self.state_machine.on_event('initiate_reconnect')
            backoff_time = self.retry_interval * (2 ** (self.retry_attempts - 1))
            self.logger.info(f"Retrying connection for {self.session_id} in {backoff_time}s (Attempt {self.retry_attempts}/{self.max_retries}).")
            await asyncio.sleep(backoff_time)
            await self.start() 
        elif self.retry_attempts >= self.max_retries:
            self.logger.error(f"Max retries reached for {self.session_id}. Connection failed.")
            self.state_machine.on_event('reconnect_failed_max_retries') 
        else:
            self.logger.info(f"Not retrying connection for {self.session_id}, current state: {self.state_machine.state.name}")

    async def handle_incoming_connection(self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter) -> None:
        client_address_info = writer.get_extra_info('peername')
        client_address = f"{client_address_info[0]}:{client_address_info[1]}" if client_address_info else "unknown client"
        self.logger.info(f"Accepted connection from {client_address} for session {self.session_id}.")
        self.logger.debug(f"FixEngine.handle_incoming_connection: reader_id={id(reader)}, writer_id={id(writer)} for client {client_address}")
        self.incoming_buffer = b"" 
        try:
            await self.network.set_transport(reader, writer)
            self.state_machine.on_event('client_accepted_awaiting_logon') 
            self.retry_attempts = 0 
            self.logger.info(f"Transport set for {client_address}. Waiting for Logon...")
            await self.receive_message()
        except ConnectionResetError:
            self.logger.warning(f"Connection reset by {client_address} during session {self.session_id}.")
        except asyncio.IncompleteReadError:
            self.logger.warning(f"Incomplete read from {client_address} (session {self.session_id}), connection closed prematurely.")
        except Exception as e:
            self.logger.error(f"Error handling connection from {client_address} (session {self.session_id}): {e}", exc_info=True)
        finally:
            self.logger.info(f"Cleaning up connection with {client_address} (session {self.session_id}).")
            if writer and not writer.is_closing():
                try:
                    writer.close()
                    await writer.wait_closed()
                except Exception as e_close:
                    self.logger.error(f"Error during writer.wait_closed() for {client_address}: {e_close}")
            
            if self.state_machine.state.name != "DISCONNECTED":
                 self.logger.info(f"Setting state to DISCONNECTED after client {client_address} handling ended (main session: {self.session_id})")
                 self.state_machine.on_event('disconnect') 
            self.logger.info(f"Connection with {client_address} (session {self.session_id}) ended.")

    async def logon(self) -> None: 
        if self.mode == 'acceptor':
            self.logger.critical("CRITICAL: Acceptor should not call FixEngine.logon().")
            return

        if self.state_machine.state.name != "LOGON_IN_PROGRESS" :
            self.logger.warning(f"Cannot initiate logon for {self.session_id}: State is {self.state_machine.state.name} (expected LOGON_IN_PROGRESS). Aborting logon.")
            if self.state_machine.state.name != "DISCONNECTED":
                 await self.disconnect(graceful=False)
            return

        try:
            logon_message = self.fixmsg({})

            reset_seq_num_flag_config = self.config_manager.get('FIX', 'reset_seq_num_on_logon', 'false').lower() == 'true'

            if reset_seq_num_flag_config:
                self.logger.info(f"ResetSeqNumFlag is true for {self.session_id}. Resetting sequence numbers to 1.")
                await self.reset_sequence_numbers() 
                logon_message[141] = 'Y'
            else:
                logon_message[141] = 'N'

            logon_message.update({ 35: 'A', 108: self.heartbeat_interval })
            encrypt_method = int(self.config_manager.get('FIX', 'encryptmethod', '0'))
            logon_message[98] = encrypt_method

            self.logger.info(f"Initiator {self.session_id} sending Logon (ResetSeqNumFlag={logon_message[141]}).")
            await self.send_message(logon_message) 
            self.logger.info(f"Initiator {self.session_id} Logon sent (SeqNum {logon_message.get(34)}). Waiting for response. Starting heartbeat mechanism.")
            if self.heartbeat: await self.heartbeat.start() 

        except Exception as e:
            self.logger.error(f"Failed to send Logon for {self.session_id}: {e}", exc_info=True)
            self.state_machine.on_event('logon_failed') 
            await self.disconnect(graceful=False) 

    async def send_message(self, message: FixMessage) -> None:
        current_state_name = self.state_machine.state.name
        can_send_logon = (message.get(35) == 'A' and self.mode == 'initiator' and current_state_name == "LOGON_IN_PROGRESS")

        if current_state_name == "DISCONNECTED" and not can_send_logon:
            self.logger.warning(f"Cannot send message (type {message.get(35)}) for {self.session_id}: Session is DISCONNECTED and not a permissible Logon.")
            return

        message[49] = self.sender
        message[56] = self.target
        message[52] = datetime.now(timezone.utc).strftime('%Y%m%d-%H:%M:%S.%f')[:-3]

        is_reset_logon = message.get(35) == 'A' and message.get(141) == 'Y'

        if 34 not in message: 
            if is_reset_logon:
                message[34] = 1
            else:
                message[34] = self.message_store.get_next_outgoing_sequence_number()
    
        wire_message = message.to_wire(codec=self.codec)
        try:
            await self.network.send(wire_message)
            await self.message_store.store_message(
                self.version, self.sender, self.target,
                message[34], 
                wire_message.decode(errors='replace')
            )
            
            if hasattr(self.message_store, 'increment_outgoing_sequence_number'):
                if not is_reset_logon:
                    await self.message_store.increment_outgoing_sequence_number()
                else: 
                    await self.message_store.set_outgoing_sequence_number(2)
            elif not is_reset_logon:
                self.logger.debug("MessageStore does not have increment_outgoing_sequence_number. Assuming get_next or internal logic handles it.")

            self.logger.info(f"Sent ({self.session_id}): {message.get(35)} (SeqNum {message.get(34)})")
            if message.get(35) != '0' or self.logger.isEnabledFor(logging.DEBUG):
                 self.logger.debug(f"Sent Details ({self.session_id}): {str(message)}")

        except ConnectionResetError as e:
            self.logger.error(f"ConnectionResetError for {self.session_id} while sending (type {message.get(35)}, seq {message.get(34)}): {e}")
            self.state_machine.on_event('disconnect') 
            await self.disconnect(graceful=False) 
        except Exception as e: 
            self.logger.error(f"Exception for {self.session_id} while sending (type {message.get(35)}, seq {message.get(34)}): {e}", exc_info=True)
            self.state_machine.on_event('disconnect') 
            await self.disconnect(graceful=False)

    async def receive_message(self) -> None:
        if not self.network.running:
            self.logger.warning(f"Network not running for {self.session_id} at start of receive_message. Ensuring disconnect state.")
            if self.state_machine.state.name != "DISCONNECTED":
                self.state_machine.on_event('disconnect') 
                await self.disconnect(graceful=False) 
            return
        try:
            await self.network.receive(self.on_network_data) 
        except Exception as e:
            self.logger.error(f"Unexpected error in FixEngine's receive_message for {self.session_id}: {e}", exc_info=True)
            if self.state_machine.state.name != "DISCONNECTED":
                 self.state_machine.on_event('disconnect')
                 await self.disconnect(graceful=False)
        finally:
            self.logger.info(f"FixEngine's receive_message loop for {self.session_id} has ended.")

    async def on_network_data(self, data_bytes: bytes) -> None:
        self.incoming_buffer += data_bytes
        self.logger.debug(f"Received {len(data_bytes)} bytes. Buffer size: {len(self.incoming_buffer)}")
        
        while True:
            try:
                begin_string_index = self.incoming_buffer.find(b"8=FIX")
                if begin_string_index == -1:
                    self.logger.debug("No '8=FIX' found in buffer. Waiting for more data.")
                    if len(self.incoming_buffer) > 8192:
                        self.logger.error("Buffer grew very large without '8=FIX'. Discarding buffer.")
                        self.incoming_buffer = b""
                    break 

                if begin_string_index > 0:
                    self.logger.warning(f"Discarding {begin_string_index} bytes of garbage data before '8=FIX': {self.incoming_buffer[:begin_string_index].decode(errors='replace')}")
                    self.incoming_buffer = self.incoming_buffer[begin_string_index:]

                soh = b'\x01'
                body_length_tag_index = self.incoming_buffer.find(soh + b"9=")
                if body_length_tag_index == -1:
                    self.logger.debug("Found '8=FIX' but no '9=' (BodyLength) yet. Buffer might be incomplete.")
                    if len(self.incoming_buffer) > 4096: 
                        self.logger.error("Buffer too large without BodyLength after '8=FIX'. Discarding buffer segment.")
                        self.incoming_buffer = self.incoming_buffer[self.incoming_buffer.find(b"8=FIX", 1):] if b"8=FIX" in self.incoming_buffer[1:] else b""
                    break 

                body_length_value_start = body_length_tag_index + len(soh + b"9=")
                body_length_value_end = self.incoming_buffer.find(soh, body_length_value_start)
                if body_length_value_end == -1:
                    self.logger.debug("Found '9=' but no SOH after its value. Buffer might be incomplete.")
                    if len(self.incoming_buffer) > 4096:
                        self.logger.error("Buffer too large without SOH after BodyLength value. Discarding.")
                        self.incoming_buffer = b""
                    break
                
                body_length_str = self.incoming_buffer[body_length_value_start:body_length_value_end]
                if not body_length_str.isdigit():
                    self.logger.error(f"Invalid BodyLength value: '{body_length_str.decode(errors='replace')}'. Discarding buffer and attempting resync.")
                    self.incoming_buffer = self.incoming_buffer[body_length_value_end:]
                    continue 
                body_length = int(body_length_str)

                body_starts_after_bodylength_field_soh = body_length_value_end + 1
                
                checksum_field_len = 7 
                
                message_end_index = body_starts_after_bodylength_field_soh + body_length + checksum_field_len
                                
                if len(self.incoming_buffer) < message_end_index:
                    self.logger.debug(f"Buffer has {len(self.incoming_buffer)} bytes, need {message_end_index} for full message (BodyLength {body_length}). Waiting for more data.")
                    break 

                full_fix_message_bytes = self.incoming_buffer[:message_end_index]
                
                if not full_fix_message_bytes.endswith(soh):
                    self.logger.error(f"Framed message does not end with SOH. Likely framing error or malformed message. Discarding: {full_fix_message_bytes.decode(errors='replace')[:100]}")
                    next_begin_string_index = self.incoming_buffer.find(b"8=FIX", 1)
                    if next_begin_string_index != -1:
                        self.incoming_buffer = self.incoming_buffer[next_begin_string_index:]
                    else:
                        self.incoming_buffer = b""
                    continue

                await self.process_single_fix_message(full_fix_message_bytes)
                self.incoming_buffer = self.incoming_buffer[message_end_index:]
                if self.incoming_buffer:
                    self.logger.debug(f"Processed one message. Remaining in buffer: {len(self.incoming_buffer)} bytes.")
                else:
                    self.logger.debug("Processed one message. Buffer is now empty.")

            except Exception as e_frame:
                self.logger.error(f"Error during message framing: {e_frame}", exc_info=True)
                self.incoming_buffer = b"" 
                break

    async def process_single_fix_message(self, message_bytes: bytes) -> None:
        full_fix_string = message_bytes.decode(errors='replace')
        self.logger.debug(f"Attempting to parse full_fix_string: '{full_fix_string}'")
        parsed_message = None
        try:
            msg_obj = self.fixmsg({})
            msg_obj.from_wire(full_fix_string, codec=self.codec)
            parsed_message = msg_obj
        except Exception as e:
            self.logger.error(f"PARSE ERROR ({self.session_id}): '{full_fix_string[:150]}...' Error: {e}", exc_info=True)
            return

        if parsed_message is None:
            self.logger.error(f"PARSING FAILED for full_fix_string: '{full_fix_string}'. from_wire returned None.")
            return

        async with self.lock:
            msg_type = parsed_message.get(35)
            received_seq_num_str = parsed_message.get(34)
            if not received_seq_num_str or not received_seq_num_str.isdigit():
                reason = f"Invalid or missing MsgSeqNum (34) in msg from {parsed_message.get(49, 'UNKNOWN')}: '{received_seq_num_str}'"
                self.logger.error(reason)
                await self.send_logout_message(text=reason)
                self.state_machine.on_event('disconnect')
                return

            received_seq_num = int(received_seq_num_str)
            expected_seq_num = self.message_store.get_next_incoming_sequence_number()

            if msg_type == '4' and parsed_message.get(123) == 'Y':
                new_seq_no = parsed_message.get(36)
                if new_seq_no and new_seq_no.isdigit():
                    new_seq_no = int(new_seq_no)
                    if new_seq_no > expected_seq_num:
                        self.logger.info(f"Processing SequenceReset-GapFill. Setting next expected incoming to {new_seq_no}.")
                        await self.message_store.set_incoming_sequence_number(new_seq_no)
                    else:
                        self.logger.info(f"Ignoring duplicate or out-of-order SequenceReset-GapFill with NewSeqNo={new_seq_no}.")
                return

            if msg_type == 'A' and parsed_message.get(141) == 'Y':
                self.logger.info("Processing Logon with ResetSeqNumFlag=Y. Setting next expected incoming to 2.")
                await self.message_store.set_incoming_sequence_number(2)
                await self.message_processor.process_message(parsed_message)
                return

            if received_seq_num < expected_seq_num:
                poss_dup_flag = parsed_message.get(43)
                if poss_dup_flag == 'Y':
                    self.logger.info(f"PossDup {msg_type} (Seq {received_seq_num}) rcvd for {self.session_id} (expected {expected_seq_num}). App layer should handle.")
                else:
                    text = f"MsgSeqNum too low, expected {expected_seq_num} but received {received_seq_num}"
                    self.logger.error(f"{text} for {self.session_id}. Not PossDup. Sending Logout.")
                    await self.send_logout_message(text=text)
                    return
            elif received_seq_num > expected_seq_num:
                if not self.resend_request_outstanding or self.resend_request_expected_seq != expected_seq_num:
                    self.logger.warning(f"MsgSeqNum TOO HIGH (Gap) for {self.session_id}. Expected: {expected_seq_num}, Rcvd: {received_seq_num}. Sending Resend Request.")
                    resend_req = self.fixmsg({35: '2', 7: expected_seq_num, 16: 0})
                    await self.send_message(resend_req)
                    self.resend_request_outstanding = True
                    self.resend_request_expected_seq = expected_seq_num
                else:
                    self.logger.debug(f"ResendRequest already outstanding for expected_seq_num={expected_seq_num}, not sending another.")
                return

            if self.resend_request_outstanding and received_seq_num == expected_seq_num:
                self.logger.debug("Gap filled, clearing resend_request_outstanding flag.")
                self.resend_request_outstanding = False
                self.resend_request_expected_seq = None

            await self.message_store.store_message(
                self.version, parsed_message.get(49), parsed_message.get(56),
                received_seq_num,
                full_fix_string
            )

            if msg_type not in ['A', '5', '4']:
                if hasattr(self.message_store, 'increment_incoming_sequence_number'):
                    await self.message_store.increment_incoming_sequence_number()
                else:
                    await self.message_store.set_incoming_sequence_number(received_seq_num + 1)

            await self.message_processor.process_message(parsed_message)

    async def send_reject_message(
        self,
        ref_seq_num: int,
        ref_tag_id: Optional[int],
        session_reject_reason: Optional[int],
        text: str,
        ref_msg_type: Optional[str] = None
    ) -> None:
        self.logger.info(f"Preparing Session Reject for {self.session_id}: RefSeq={ref_seq_num}, RefTag={ref_tag_id}, Reason={session_reject_reason}, Text='{text}'")
        reject_msg = self.fixmsg({
            35: '3', 45: ref_seq_num, 58: text
        })
        if ref_tag_id: reject_msg[371] = ref_tag_id
        if ref_msg_type: reject_msg[372] = ref_msg_type
        if session_reject_reason is not None: reject_msg[373] = session_reject_reason
        await self.send_message(reject_msg)

    async def disconnect(self, graceful: bool = True) -> None:
        current_state_name = self.state_machine.state.name
        self.logger.info(f"Disconnect requested for {self.session_id}. Graceful: {graceful}. Current state: {current_state_name}.")
        self.incoming_buffer = b"" 

        if current_state_name == "DISCONNECTED" and (not self.network or not self.network.running):
            self.logger.info(f"Session {self.session_id} already disconnected and network not running.")
            return
        
        if graceful and current_state_name == "ACTIVE":
            self.state_machine.on_event('logout_initiated') 
            try:
                self.logger.info(f"Attempting graceful logout for {self.session_id}.")
                await self.send_logout_message() 
            except Exception as e_logout:
                self.logger.error(f"Error sending logout for {self.session_id} during graceful disconnect: {e_logout}")
        else:
            if current_state_name != "DISCONNECTED":
                 self.state_machine.on_event('disconnect')

        if self.network: 
            self.logger.debug(f"Disconnecting network layer for {self.session_id}.")
            await self.network.disconnect() 

        if self.heartbeat and self.heartbeat.is_running():
            self.logger.debug(f"Stopping heartbeat for {self.session_id}.")
            await self.heartbeat.stop()

        if self.scheduler_task and not self.scheduler_task.done():
            self.logger.debug(f"Cancelling scheduler task for {self.session_id}.")
            self.scheduler_task.cancel()
            try:
                await self.scheduler_task
            except asyncio.CancelledError:
                self.logger.info(f"Scheduler task for {self.session_id} cancelled.")
            self.scheduler_task = None

        if self.message_store and hasattr(self.message_store, 'close'):
            await self.message_store.close()

        self.retry_attempts = 0 
        
        if self.state_machine.state.name != "DISCONNECTED":
            self.logger.warning(f"State was {self.state_machine.state.name} after disconnect ops. Forcing to DISCONNECTED.")
            self.state_machine.on_event('force_disconnect') 

        self.logger.info(f"FIX Engine {self.session_id} disconnected operations complete.")

    async def reset_sequence_numbers(self) -> None:
        self.logger.info(f"Resetting sequence numbers to 1 for {self.session_id} (both inbound and outbound).")
        if self.message_store:
            await self.message_store.reset_sequence_numbers()

    async def set_inbound_sequence_number(self, seq_num: int) -> None:
        self.logger.info(f"Externally setting inbound sequence for {self.session_id} to {seq_num}.")
        if self.message_store:
            await self.message_store.set_incoming_sequence_number(seq_num)

    async def set_outbound_sequence_number(self, seq_num: int) -> None:
        self.logger.info(f"Externally setting outbound sequence for {self.session_id} to {seq_num}.")
        if self.message_store:
            await self.message_store.set_outgoing_sequence_number(seq_num)

    async def set_sequence_numbers(self, incoming_seqnum: int, outgoing_seqnum: int) -> None:
        self.logger.info(f"Externally setting both inbound ({incoming_seqnum}) and outbound ({outgoing_seqnum}) sequence numbers for {self.session_id}.")
        if self.message_store:
            await self.message_store.set_incoming_sequence_number(incoming_seqnum)
            await self.message_store.set_outgoing_sequence_number(outgoing_seqnum)

    async def send_logout_message(self, text: str = "Operator requested logout") -> None:
        current_state_name = self.state_machine.state.name
        
        if current_state_name == "DISCONNECTED":
            self.logger.info(f"Cannot send Logout for {self.session_id}: Already disconnected.")
            return
        
        if current_state_name == "LOGOUT_IN_PROGRESS":
            self.logger.info(f"Logout already in progress for {self.session_id}. New logout request ignored.")
            return

        self.logger.info(f"Sending Logout for {self.session_id}. Text: '{text}'")
        logout_msg = self.fixmsg({ 35: '5', 58: text })

        try:
            if current_state_name == "ACTIVE":
                 self.state_machine.on_event('logout_initiated') 
            await self.send_message(logout_msg) 
            self.logger.info(f"Successfully sent Logout (Seq {logout_msg.get(34)}) for {self.session_id}.")
        except Exception as e:
            self.logger.error(f"Failed to send Logout for {self.session_id}: {e}", exc_info=True)
            if self.state_machine.state.name != "DISCONNECTED": 
                 self.logger.debug(f"Ensuring disconnect due to send_logout_message failure (current state: {current_state_name}).")
                 if self.state_machine.state.name != "DISCONNECTED": 
                    self.state_machine.on_event('disconnect') 
                    await self.disconnect(graceful=False)

    async def request_logoff(self, timeout: float = 10.0) -> None:
        self.logger.info(f"request_logoff() called for {self.session_id}")
        if self.state_machine.state.name not in ("ACTIVE", "LOGOUT_IN_PROGRESS"):
            self.logger.warning(f"Cannot request logoff: session state is {self.state_machine.state.name}")
            return

        self._logoff_future = asyncio.get_event_loop().create_future()
        await self.send_logout_message("Operator requested logout")

        try:
            await asyncio.wait_for(self._logoff_future, timeout=timeout)
            self.logger.info(f"Logoff response received for {self.session_id}. Proceeding to disconnect.")
        except asyncio.TimeoutError:
            self.logger.warning(f"Timeout waiting for Logoff response for {self.session_id}. Forcing disconnect.")

        await self.disconnect(graceful=False)

    def notify_logoff_received(self) -> None:
        if hasattr(self, "_logoff_future") and self._logoff_future and not self._logoff_future.done():
            self._logoff_future.set_result(True)
            self.logger.debug(f"notify_logoff_received: Logoff future set for {self.session_id}")

    @classmethod
    async def create(
        cls,
        config_manager: ConfigManager,
        application: Any,
        initial_incoming_seqnum: Optional[int] = None,
        initial_outgoing_seqnum: Optional[int] = None
    ) -> "FixEngine":
        self = cls(config_manager, application, initial_incoming_seqnum, initial_outgoing_seqnum)
        db_path = self.config_manager.get('FIX', 'state_file', 'fix_state.db')
        store_type = self.config_manager.get_message_store_type('database')
        self.logger.info(f"Using message store type: {store_type}")
        self.message_store = await MessageStoreFactory.get_message_store(
            store_type,
            db_path,
            beginstring=self.version,
            sendercompid=self.sender,
            targetcompid=self.target
        )
        self.message_processor.message_store = self.message_store
        for handler in self.message_processor.handlers.values():
            handler.message_store = self.message_store
        return self

    async def initialize(self) -> None:
        if self.message_store and hasattr(self.message_store, "initialize"):
            await self.message_store.initialize()
