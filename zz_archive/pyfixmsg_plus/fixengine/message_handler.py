from functools import wraps
import asyncio
import logging
from typing import Any, Callable, Awaitable, Dict

def logging_decorator(handler_func: Callable[..., Awaitable[Any]]) -> Callable[..., Awaitable[Any]]:
    @wraps(handler_func)
    async def wrapper(self, message: Any) -> Any:
        msg_type_for_log = message.get(35, "UNKNOWN_TYPE")
        seq_num_for_log = message.get(34, "NO_SEQ")
        if hasattr(self, 'logger') and self.logger:
            log_message_str = message.to_wire(self.engine.codec) if hasattr(self.engine, 'codec') and hasattr(message, 'to_wire') else str(message)
            self.logger.debug(f"Handling {msg_type_for_log} (Seq {seq_num_for_log}). Incoming: {log_message_str}")
        else:
            print(f"Fallback Logging: Handling message before {msg_type_for_log} (Seq {seq_num_for_log})")
        
        result = await handler_func(self, message)
        
        if hasattr(self, 'logger') and self.logger:
            self.logger.debug(f"Finished handling {msg_type_for_log} (Seq {seq_num_for_log}).")
        else:
            print(f"Fallback Logging: Finished handling message {msg_type_for_log} (Seq {seq_num_for_log})")
        return result
    return wrapper

class MessageHandler:
    def __init__(self, message_store: Any, state_machine: Any, application: Any, engine: Any) -> None:
        self.message_store = message_store
        self.state_machine = state_machine
        self.application = application
        self.engine = engine
        self.logger = logging.getLogger(f"{self.__class__.__module__}.{self.__class__.__name__}")

    async def handle(self, message: Any) -> Any:
        raise NotImplementedError

class LogonHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        msg_type = message.get(35)
        if msg_type != 'A':
            self.logger.error(f"LogonHandler received non-Logon message: {msg_type}")
            return

        received_sender_comp_id = message.get(49)
        received_target_comp_id = message.get(56)
        received_seq_num_str = message.get(34)
        received_heartbeat_interval_str = message.get(108)
        reset_seq_num_flag = message.get(141) == 'Y' # Check ResetSeqNumFlag

        if not received_seq_num_str or not received_seq_num_str.isdigit():
            reason = f"Invalid or missing MsgSeqNum (34) in Logon: '{received_seq_num_str}'"
            self.logger.error(reason)
            await self.engine.send_logout_message(text=reason)
            return
        received_seq_num = int(received_seq_num_str)

        if not received_heartbeat_interval_str or not received_heartbeat_interval_str.isdigit():
            reason = f"Invalid or missing HeartBtInt (108) in Logon: '{received_heartbeat_interval_str}'"
            self.logger.error(reason)
            await self.engine.send_logout_message(text=reason)
            return
        received_heartbeat_interval = int(received_heartbeat_interval_str)

        if self.engine.mode == 'acceptor':
            self.logger.info(f"Acceptor ({self.engine.sender}) received Logon from Initiator ({received_sender_comp_id}): Seq={received_seq_num}, HBInt={received_heartbeat_interval}, ResetFlag={reset_seq_num_flag}")

            if received_sender_comp_id != self.engine.target or received_target_comp_id != self.engine.sender:
                reason = f"Invalid CompIDs in Logon. Expected Sender={self.engine.target}/Target={self.engine.sender}. Got Sender={received_sender_comp_id}/Target={received_target_comp_id}"
                self.logger.error(reason)
                await self.engine.send_logout_message(text=reason)
                return

            expected_incoming_seq_num = self.message_store.get_next_incoming_sequence_number()

            if reset_seq_num_flag:
                self.logger.info(f"ResetSeqNumFlag=Y received from {received_sender_comp_id}. Acceptor will reset its sequence numbers.")
                if received_seq_num != 1:
                    reason = f"Invalid Logon: ResetSeqNumFlag(141)=Y but MsgSeqNum(34)={received_seq_num} (expected 1)."
                    self.logger.error(reason)
                    await self.engine.send_logout_message(text=reason)
                    return
                await self.engine.reset_sequence_numbers()
                expected_incoming_seq_num = 1
            
            if received_seq_num < expected_incoming_seq_num:
                reason = f"MsgSeqNum too low in Logon. Expected >= {expected_incoming_seq_num}, Got {received_seq_num}."
                self.logger.error(reason)
                await self.engine.send_logout_message(text=reason)
                return
            elif received_seq_num > expected_incoming_seq_num:
                reason = f"MsgSeqNum too high in Logon. Expected {expected_incoming_seq_num}, Got {received_seq_num}."
                self.logger.error(reason)
                await self.engine.send_logout_message(text=reason)
                return
            
            self.logger.info(f"Acceptor: Incoming Logon from {received_sender_comp_id} is valid (SeqNum {received_seq_num}).")
            self.engine.heartbeat.set_remote_interval(received_heartbeat_interval)

            acceptor_logon_response = self.engine.fixmsg({})
            acceptor_logon_response.update({ 35: 'A', 108: self.engine.heartbeat_interval })
            if reset_seq_num_flag:
                acceptor_logon_response[141] = 'Y'
            
            await self.engine.send_message(acceptor_logon_response)
            self.logger.info(f"Acceptor sent Logon response to {received_sender_comp_id} (SeqNum {acceptor_logon_response.get(34)}).")

            self.state_machine.on_event('logon_received_valid') 
            if self.engine.heartbeat: await self.engine.heartbeat.start()

        elif self.engine.mode == 'initiator':
            self.logger.info(f"Initiator ({self.engine.sender}) received Logon response from Acceptor ({received_sender_comp_id}): Seq={received_seq_num}, HBInt={received_heartbeat_interval}, ResetFlag={reset_seq_num_flag}")

            if received_sender_comp_id != self.engine.target or received_target_comp_id != self.engine.sender:
                reason = f"Invalid CompIDs in Logon response. Expected Sender={self.engine.target}/Target={self.engine.sender}. Got Sender={received_sender_comp_id}/Target={received_target_comp_id}"
                self.logger.error(reason)
                await self.engine.send_logout_message(text=reason)
                return

            expected_incoming_seq_num = self.message_store.get_next_incoming_sequence_number()
            our_logon_had_reset = self.engine.config_manager.get('FIX', 'reset_seq_num_on_logon', 'false').lower() == 'true'

            if our_logon_had_reset and reset_seq_num_flag and received_seq_num == 1:
                # Accept Logon response with MsgSeqNum=1 if we sent ResetSeqNumFlag=Y
                self.logger.info("Initiator: Accepting Logon response with MsgSeqNum=1 due to ResetSeqNumFlag=Y.")
            else:
                if received_seq_num < expected_incoming_seq_num:
                    reason = f"MsgSeqNum too low in Logon response. Expected >= {expected_incoming_seq_num}, Got {received_seq_num}."
                    self.logger.error(reason)
                    await self.engine.send_logout_message(text=reason)
                    return
                elif received_seq_num > expected_incoming_seq_num:
                    reason = f"MsgSeqNum too high in Logon response. Expected {expected_incoming_seq_num}, Got {received_seq_num}."
                    self.logger.error(reason)
                    await self.engine.send_logout_message(text=reason)
                    return

            self.logger.info(f"Initiator: Logon response from {received_sender_comp_id} is valid (SeqNum {received_seq_num}).")
            self.engine.heartbeat.set_remote_interval(received_heartbeat_interval)
            self.state_machine.on_event('logon_successful') 
        else:
            self.logger.error(f"LogonHandler: Unknown engine mode '{self.engine.mode}'")


class TestRequestHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        test_req_id = message.get(112)
        if not test_req_id:
            self.logger.warning("Received TestRequest without TestReqID (112). Cannot properly respond.")
            await self.engine.send_reject_message(
                ref_seq_num=message.get(34), 
                ref_tag_id=112, 
                session_reject_reason=1, 
                text="TestRequest missing TestReqID(112)",
                ref_msg_type=message.get(35)
            )
            return

        self.logger.info(f"Received TestRequest (112={test_req_id}). Responding with Heartbeat.")
        heartbeat_response = self.engine.fixmsg({
            35: '0', 112: test_req_id
        })
        await self.engine.send_message(heartbeat_response)
        self.logger.info(f"Sent Heartbeat in response to TestRequest {test_req_id}.")


class ExecutionReportHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        await self.application.onMessage(message, self.engine.session_id)

class NewOrderHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        await self.application.onMessage(message, self.engine.session_id)

class CancelOrderHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        await self.application.onMessage(message, self.engine.session_id)

class OrderCancelReplaceHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        await self.application.onMessage(message, self.engine.session_id)

class OrderCancelRejectHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        await self.application.onMessage(message, self.engine.session_id)

class NewOrderMultilegHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        await self.application.onMessage(message, self.engine.session_id)

class MultilegOrderCancelReplaceHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        await self.application.onMessage(message, self.engine.session_id)

class ResendRequestHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        start_seq_num_str = message.get(7)
        end_seq_num_str = message.get(16)

        if not start_seq_num_str or not start_seq_num_str.isdigit() or \
           not end_seq_num_str or not end_seq_num_str.isdigit():
            reason = f"Invalid BeginSeqNo(7) or EndSeqNo(16) in ResendRequest: Begin='{start_seq_num_str}', End='{end_seq_num_str}'"
            self.logger.error(reason)
            await self.engine.send_reject_message(
                ref_seq_num=message.get(34), 
                ref_tag_id=7 if not start_seq_num_str or not start_seq_num_str.isdigit() else 16,
                session_reject_reason=5, 
                text=reason,
                ref_msg_type=message.get(35)
            )
            return

        start_seq_num = int(start_seq_num_str)
        end_seq_num = int(end_seq_num_str)
        
        self.logger.info(f"Received Resend Request: BeginSeqNo={start_seq_num}, EndSeqNo={end_seq_num}.")

        if end_seq_num != 0 and end_seq_num < start_seq_num:
            reason = f"Invalid range in ResendRequest: EndSeqNo({end_seq_num}) < BeginSeqNo({start_seq_num})"
            self.logger.error(reason)
            await self.engine.send_reject_message(
                 ref_seq_num=message.get(34), ref_tag_id=16, session_reject_reason=5, text=reason, ref_msg_type=message.get(35)
            )
            return

        effective_end_seq_num = end_seq_num
        if end_seq_num == 0:
            current_outgoing = self.message_store.get_current_outgoing_sequence_number() 
            effective_end_seq_num = current_outgoing
            self.logger.info(f"EndSeqNo=0, adjusted to current last sent: {effective_end_seq_num}.")
            if effective_end_seq_num < start_seq_num and start_seq_num > 0 :
                 self.logger.info(f"Adjusted EndSeqNo ({effective_end_seq_num}) is less than BeginSeqNo ({start_seq_num}). Nothing to resend.")
                 self.logger.info(f"Completed processing Resend Request from {start_seq_num} to {end_seq_num} (effective {effective_end_seq_num}). Nothing to resend.")
                 return

        for seq_num_to_resend in range(start_seq_num, effective_end_seq_num + 1):
            stored_message_str = await self.message_store.get_message(
                self.engine.version,
                self.engine.sender, 
                self.engine.target, 
                seq_num_to_resend
            )

            if stored_message_str:
                self.logger.info(f"Resending stored message for SeqNum {seq_num_to_resend}.")
                try:
                    resent_msg = self.engine.fixmsg({}).from_wire(stored_message_str, codec=self.engine.codec)
                    resent_msg[43] = 'Y' 
                    original_sending_time = resent_msg.get(52) 
                    if original_sending_time:
                        resent_msg[122] = original_sending_time
                    await self.engine.send_message(resent_msg)
                except Exception as e:
                    self.logger.error(f"Error parsing or preparing stored message {seq_num_to_resend} for resend: {e}. Sending GapFill.", exc_info=True)
                    await self.send_gap_fill(seq_num_to_resend, seq_num_to_resend)
            else:
                self.logger.warning(f"Message for SeqNum {seq_num_to_resend} not found in store. Sending GapFill.")
                await self.send_gap_fill(seq_num_to_resend, seq_num_to_resend)
                # Always call application callback

        if hasattr(self.application, "fromAdmin"):
            await self.application.fromAdmin(message, self.engine.session_id)
        self.logger.info(f"Completed processing Resend Request from {start_seq_num} to {end_seq_num} (effective {effective_end_seq_num}).")

    async def send_gap_fill(self, begin_gap_seq_num: int, end_gap_seq_num: int) -> None:
        next_seq_no_after_gap = end_gap_seq_num + 1
        self.logger.info(f"Sending SequenceReset-GapFill for range {begin_gap_seq_num}-{end_gap_seq_num}. NewSeqNo will be {next_seq_no_after_gap}.")
        gap_fill_msg = self.engine.fixmsg({})
        gap_fill_msg.update({ 35: '4', 36: next_seq_no_after_gap, 123: 'Y' })
        await self.engine.send_message(gap_fill_msg)


class SequenceResetHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        gap_fill_flag = message.get(123) == 'Y'
        new_seq_no_str = message.get(36)
        msg_seq_num_header_str = message.get(34)

        if not new_seq_no_str or not new_seq_no_str.isdigit():
            reason = f"Invalid or missing NewSeqNo(36) in SequenceReset: '{new_seq_no_str}'"
            self.logger.error(reason)
            await self.engine.send_reject_message(message.get(34) or 0, 36, 5, reason, message.get(35))
            return
        new_seq_no = int(new_seq_no_str)
        msg_seq_num_header = int(msg_seq_num_header_str) 

        self.logger.info(f"Received SequenceReset: NewSeqNo={new_seq_no}, GapFillFlag={gap_fill_flag}, HeaderSeqNum={msg_seq_num_header}")
        current_expected_incoming = self.message_store.get_next_incoming_sequence_number()

        if new_seq_no <= msg_seq_num_header and not gap_fill_flag:
             self.logger.warning(f"SequenceReset-Reset: NewSeqNo ({new_seq_no}) must be greater than its own header MsgSeqNum ({msg_seq_num_header}).")
             await self.engine.send_reject_message(msg_seq_num_header, 36, 5, "NewSeqNo must be > MsgSeqNum for SequenceReset-Reset", message.get(35))
             return

        if not gap_fill_flag: 
            if new_seq_no <= current_expected_incoming:
                reason = f"SequenceReset-Reset: NewSeqNo ({new_seq_no}) must be greater than current expected incoming ({current_expected_incoming})."
                self.logger.error(reason)
                await self.engine.send_reject_message(msg_seq_num_header, 36, 5, reason, message.get(35))
                return
            
            self.logger.info(f"Processing SequenceReset-Reset. Setting next incoming and outgoing to {new_seq_no}.")
            await self.message_store.set_incoming_sequence_number(new_seq_no)
            await self.message_store.set_outgoing_sequence_number(new_seq_no) 
            self.logger.info(f"Both incoming and outgoing sequence numbers reset to {new_seq_no}.")
            await self.engine.send_message(self.engine.fixmsg({35: '0'}))

        else: 
            if new_seq_no <= current_expected_incoming:
                self.logger.warning(f"SequenceReset-GapFill: NewSeqNo ({new_seq_no}) is not greater than current expected incoming ({current_expected_incoming}).")
                if new_seq_no <= msg_seq_num_header:
                     await self.engine.send_reject_message(msg_seq_num_header, 36, 5, "NewSeqNo in GapFill must be > MsgSeqNum of SequenceReset itself", message.get(35))
                     return
            
            self.logger.info(f"Processing SequenceReset-GapFill. Setting next expected incoming to {new_seq_no}.")
            await self.message_store.set_incoming_sequence_number(new_seq_no)

            # Always call application callback
            if hasattr(self.application, "fromAdmin"):
                await self.application.fromAdmin(message, self.engine.session_id)

class RejectHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        ref_seq_num = message.get(45) 
        ref_tag_id = message.get(371)
        ref_msg_type = message.get(372)
        reject_reason_code = message.get(373) 
        text = message.get(58) 
        self.logger.warning(f"Received Session-Level Reject: RefSeqNum={ref_seq_num}, RefTagID={ref_tag_id}, RefMsgType={ref_msg_type}, ReasonCode={reject_reason_code}, Text='{text}'")
        
        # Always call application callback
        if hasattr(self.application, "fromAdmin"):
            await self.application.fromAdmin(message, self.engine.session_id)       
        if hasattr(self.application, 'onReject'):
            await self.application.onReject(message)


class LogoutHandler(MessageHandler): 
    @logging_decorator
    async def handle(self, message: Any) -> None:
        text = message.get(58, "")
        received_seq_num = message.get(34)
        self.logger.info(f"Logout (Seq={received_seq_num}) received from counterparty. Text: '{text}'.")
        
        current_engine_state = self.state_machine.state.name
        if current_engine_state not in ['DISCONNECTED', 'LOGOUT_IN_PROGRESS']:
            self.logger.info(f"Session was {current_engine_state}. Sending confirming Logout.")
            await self.engine.send_logout_message(text="Logout Acknowledged by Handler")
        else:
            self.logger.info(f"Session already in state {current_engine_state}. Not sending confirming Logout from handler.")
        
        await self.engine.disconnect(graceful=False) 
        self.logger.info("LogoutHandler initiated engine disconnect to ensure cleanup.")

        if hasattr(self.engine, "notify_logoff_received"):
            self.engine.notify_logoff_received()
        # Always call application callback
        if hasattr(self.application, "fromAdmin"):
            await self.application.fromAdmin(message, self.engine.session_id)


class HeartbeatHandler(MessageHandler):
    @logging_decorator
    async def handle(self, message: Any) -> None:
        test_req_id_in_hb = message.get(112) 
        self.logger.debug(f"Received Heartbeat. TestReqID (112) in Heartbeat: {test_req_id_in_hb}")
        
        if self.engine.heartbeat:
            self.engine.heartbeat.process_incoming_heartbeat(test_req_id_in_hb)
        else:
            self.logger.warning("Received Heartbeat, but FixEngine.heartbeat object is not available.")
        # Always call application callback
        if hasattr(self.application, "fromAdmin"):
            await self.application.fromAdmin(message, self.engine.session_id)

class MessageProcessor:
    def __init__(self, message_store: Any, state_machine: Any, application: Any, engine: Any) -> None:
        self.handlers: Dict[str, MessageHandler] = {}
        self.message_store = message_store
        self.state_machine = state_machine
        self.application = application
        self.engine = engine 
        self.logger = logging.getLogger(self.__class__.__name__)


    def register_handler(self, message_type: str, handler_instance: MessageHandler) -> None:
        self.handlers[message_type] = handler_instance
        self.logger.debug(f"Registered handler for message type '{message_type}': {handler_instance.__class__.__name__}")

    async def process_message(self, message: Any) -> None:
        message_type = message.get(35)
        handler = self.handlers.get(message_type)
        if handler:
            await handler.handle(message) 
        else:
            ref_seq_num_str = message.get(34)
            log_msg_content = message.to_wire(self.engine.codec) if hasattr(self.engine, 'codec') and hasattr(message, 'to_wire') else str(message)
            self.logger.warning(f"No handler registered for message type: {message_type}. Message SeqNum: {ref_seq_num_str}. Content: {log_msg_content[:200]}...") # Limit log length

            if self.state_machine.state.name == 'ACTIVE': # Assuming Active.name or direct string compare
                is_admin_type = (len(message_type) == 1 and (message_type.isupper() or message_type.isdigit())) or \
                                (len(message_type) == 2 and message_type.isupper() and message_type[0] == 'A') 

                if is_admin_type: 
                    self.logger.error(f"Unhandled Admin Message Type: {message_type}. Sending Reject.")
                    await self.engine.send_reject_message(
                        ref_seq_num=ref_seq_num_str if ref_seq_num_str and ref_seq_num_str.isdigit() else 0,
                        ref_tag_id=35, 
                        session_reject_reason=3, 
                        text=f"Unsupported admin message type: {message_type}",
                        ref_msg_type=message_type
                    )
                else: 
                    self.logger.info(f"Passing unhandled application message type {message_type} to generic application.onMessage.")
                    if hasattr(self.application, 'onMessage'): 
                        await self.application.onMessage(message)
                    else:
                        self.logger.warning(f"Application does not have a generic onMessage method for type {message_type}.")
