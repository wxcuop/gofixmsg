import logging
import asyncio
import datetime

logger = logging.getLogger(__name__)

from pyfixmsg_plus.application import Application

class DummyApplication(Application):
    def __init__(self):
        self.engine = None
        self.logoff_confirmed = False
        if hasattr(super(), '__init__'):
            super().__init__()

    def set_engine(self, engine): 
        self.engine = engine

    async def onCreate(self, sessionID):
        pass

    async def onLogon(self, sessionID, message=None): 
        logger.info(f"[{sessionID}] Pyfixmsg_plus App: Received Logon from counterparty.")

    async def onLogout(self, sessionID, message=None):
        logger.info(f"[{sessionID}] Pyfixmsg_plus App: Received Logoff from counterparty.")
        self.logoff_confirmed = True

    async def toAdmin(self, message, sessionID):
        return message, sessionID 

    async def fromAdmin(self, message, sessionID):
        return message, sessionID 

    async def toApp(self, message, sessionID):
        return message, sessionID 

    async def fromApp(self, message, sessionID):
        return message, sessionID 

    async def onMessage(self, message, sessionID):
        msg_type = message.get(35) if hasattr(message, 'get') else "Unknown"
        logger.info(f"[{sessionID}] Pyfixmsg_plus App: Received message type {msg_type}: {str(message)}")

async def run_common_initiator_logic(engine, clordid_generator, logger, max_loops_disconnected=10):
    sent_test_order = False
    sent_test_request = False
    sent_resend_request = False
    sent_reset_seq = False
    sent_cancel_order = False
    loop_count = 0

    while True:
        loop_count += 1
        if not engine or not hasattr(engine, 'state_machine'):
            logger.warning("Engine or state_machine not available yet, sleeping...")
            await asyncio.sleep(1)
            continue

        current_state = engine.state_machine.state.name
        logger.debug(f"Main loop check: Engine state is {current_state}, sent_test_order={sent_test_order}")

        if current_state == 'DISCONNECTED':
            if loop_count > max_loops_disconnected:
                logger.info(f"Engine is persistently DISCONNECTED after {loop_count} checks. Exiting example loop.")
                if hasattr(engine, 'retry_attempts') and hasattr(engine, 'max_retries') and \
                   engine.max_retries > 0 and engine.retry_attempts >= engine.max_retries:
                    logger.warning(f"Max retries ({engine.max_retries}) also reached.")
                break
            else:
                logger.info(f"Engine is DISCONNECTED (check {loop_count}/{max_loops_disconnected}). Will check again.")

        if current_state == 'ACTIVE':
            # 1. Send Heartbeat
            logger.info("Session is ACTIVE. Waiting 2 seconds before sending Heartbeat (35=0)...")
            await asyncio.sleep(2)
            heartbeat_msg = engine.fixmsg({
                35: '0',
                34: engine.message_store.get_next_outgoing_sequence_number(),
                49: engine.sender,
                56: engine.target,
            })
            logger.info(f"Sending manual Heartbeat: {str(heartbeat_msg)}")
            await engine.send_message(heartbeat_msg)

            # Wait for a bit less than the heartbeat interval before sending the next test message
            logger.info("Heartbeat sent. Waiting 28 seconds before sending TestRequest (35=1)...")
            await asyncio.sleep(28)

            # 2. Send TestRequest
            if not sent_test_request:
                test_req_id = f"TESTREQ{loop_count}"
                test_request = engine.fixmsg({
                    35: '1',
                    34: engine.message_store.get_next_outgoing_sequence_number(),
                    49: engine.sender,
                    56: engine.target,
                    112: test_req_id
                })
                logger.info(f"Sending TestRequest: {str(test_request)}")
                await engine.send_message(test_request)
                sent_test_request = True

            # Wait before sending ResendRequest
            logger.info("TestRequest sent. Waiting 10 seconds before sending ResendRequest (35=2)...")
            await asyncio.sleep(10)

            # 3. Send ResendRequest
            if not sent_resend_request:
                end_seq = engine.message_store.get_next_outgoing_sequence_number()
                resend_request = engine.fixmsg({
                    35: '2',
                    34: end_seq,
                    49: engine.sender,
                    56: engine.target,
                    7: 1,
                    16: end_seq
                })
                logger.info(f"Sending ResendRequest: {str(resend_request)}")
                await engine.send_message(resend_request)
                sent_resend_request = True

            # Wait before sending SequenceReset
            logger.info("ResendRequest sent. Waiting 10 seconds before sending SequenceReset (35=4)...")
            await asyncio.sleep(10)

            # 4. Send Sequence Reset (Gap Fill)
            if not sent_reset_seq:
                seq_reset = engine.fixmsg({
                    35: '4',
                    34: engine.message_store.get_next_outgoing_sequence_number(),
                    49: engine.sender,
                    56: engine.target,
                    36: 10,  # NewSeqNo
                    123: 'Y'  # GapFillFlag
                })
                logger.info(f"Sending SequenceReset (GapFill): {str(seq_reset)}")
                await engine.send_message(seq_reset)
                # After sending SequenceReset, set outgoing seqnum to 10
                await engine.message_store.set_outgoing_sequence_number(10)
                sent_reset_seq = True

            # Wait before sending NewOrderSingle
            logger.info("SequenceReset sent. Waiting 10 seconds before sending NewOrderSingle (35=D)...")
            await asyncio.sleep(10)

            # 5. Send NewOrderSingle
            if not sent_test_order:
                if engine.state_machine.state.name == 'ACTIVE':
                    clordid = clordid_generator.next_id()
                    test_order = engine.fixmsg({
                        35: 'D', 11: clordid, 55: 'MSFT', 54: '1', 38: '100',
                        40: '1', 44: '150.00',
                        60: datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%d-%H:%M:%S.%f')[:-3]
                    })
                    logger.info(f"Sending test NewOrderSingle: {str(test_order)}")
                    await engine.send_message(test_order)
                    sent_test_order = True

            # Wait before sending OrderCancelRequest
            logger.info("NewOrderSingle sent. Waiting 10 seconds before sending OrderCancelRequest (35=F)...")
            await asyncio.sleep(10)

            # 6. Send OrderCancelRequest
            if sent_test_order and not sent_cancel_order:
                clordid = clordid_generator.next_id()
                cancel_order = engine.fixmsg({
                    35: 'F', 11: clordid, 41: clordid, 55: 'MSFT', 54: '1', 38: '100',
                    60: datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%d-%H:%M:%S.%f')[:-3]
                })
                logger.info(f"Sending OrderCancelRequest: {str(cancel_order)}")
                await engine.send_message(cancel_order)
                sent_cancel_order = True

            # Wait before logoff
            logger.info("OrderCancelRequest sent. Waiting 10 seconds before initiating FIX Logoff...")
            await asyncio.sleep(10)

            # 7. Initiate Logoff
            if sent_test_order and sent_cancel_order:
                logger.info("Test orders sent. Initiating FIX Logoff handshake via engine.request_logoff().")
                await engine.request_logoff(timeout=10)
                break

        await asyncio.sleep(1)