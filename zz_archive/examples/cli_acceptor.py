import asyncio
import logging
import os
import datetime
from pyfixmsg_plus.fixengine.configmanager import ConfigManager
from pyfixmsg_plus.fixengine.engine import FixEngine
from pyfixmsg_plus.application import Application # Correct import path

# Configure logging
logging.basicConfig(
    level=logging.DEBUG, # <--- Ensure this is DEBUG
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    # handlers=[
    #     logging.StreamHandler() # To see logs in console if running locally
    # ]
)
logger = logging.getLogger(__name__)
class DummyApplication(Application):
    def __init__(self):
        self.engine = None
        self.logger = logging.getLogger(self.__class__.__name__)

    def set_engine(self, engine): 
        self.engine = engine
        # self.logger.info("Engine set on Acceptor DummyApplication.")


    # --- Implementation of abstract methods from pyfixmsg_plus.application.Application ---
    async def onCreate(self, sessionID):
        # self.logger.info(f"[{sessionID}] Acceptor App: onCreate callback triggered.")
        pass # Minimal implementation for the example

    async def onLogon(self, sessionID, message=None): 
        # self.logger.info(f"[{sessionID}] Acceptor App: Logon successful.")
        if message and hasattr(message, 'to_wire'):
            pass
            # self.logger.info(f"Logon Message details: {message.to_wire(pretty=True)}")

    async def onLogout(self, sessionID, message=None): 
        # self.logger.info(f"[{sessionID}] Acceptor App: Logout.")
        if message and hasattr(message, 'to_wire'):
            pass
            # self.logger.info(f"Logout Message details: {message.to_wire(pretty=True)}")

    async def toAdmin(self, message, sessionID):
        # self.logger.debug(f"[{sessionID}] Acceptor App toAdmin: MsgType {message.get(35) if hasattr(message, 'get') else 'Unknown'}")
        return message 

    async def fromAdmin(self, message, sessionID):
        # self.logger.debug(f"[{sessionID}] Acceptor App fromAdmin: MsgType {message.get(35) if hasattr(message, 'get') else 'Unknown'}")
        return message 

    async def toApp(self, message, sessionID):
        # self.logger.debug(f"[{sessionID}] Acceptor App toApp: MsgType {message.get(35) if hasattr(message, 'get') else 'Unknown'}")
        return message 

    async def fromApp(self, message, sessionID):
        # self.logger.debug(f"[{sessionID}] Acceptor App fromApp: MsgType {message.get(35) if hasattr(message, 'get') else 'Unknown'}")
        return message 

    async def onMessage(self, message, sessionID):
        msg_type = message.get(35) if hasattr(message, 'get') else "Unknown"
        # self.logger.info(f"[{sessionID}] Acceptor App: Received message type {msg_type}: {message.to_wire(pretty=True) if hasattr(message, 'to_wire') else str(message)}")
        
        if msg_type == 'D': # NewOrderSingle
            if not self.engine:
                # self.logger.error(f"[{sessionID}] Engine not set in DummyApplication. Cannot send ExecutionReport.")
                return

            cl_ord_id = message.get(11)
            symbol = message.get(55)
            side = message.get(54)
            order_qty = message.get(38)
            price = message.get(44) 

            if not all([cl_ord_id, symbol, side, order_qty]):
                # self.logger.error(f"[{sessionID}] Missing required fields in NewOrderSingle to send ExecutionReport. ClOrdID={cl_ord_id}, Symbol={symbol}, Side={side}, OrderQty={order_qty}")
                return

            exec_report = self.engine.fixmsg({
                35: '8', 11: cl_ord_id, 37: f"exec-{cl_ord_id}", 17: f"execid-{cl_ord_id}", 
                150: '0', 39: '0', 55: symbol, 54: side, 38: order_qty, 14: 0, 6: 0,
            })
            if price: 
                exec_report[44] = price
            
            # self.logger.info(f"[{sessionID}] Application: Sending ExecutionReport for {cl_ord_id}")
            await self.engine.send_message(exec_report)


class AcceptorApplication(DummyApplication):
    def __init__(self):
        super().__init__()
        self.logon_received = False
        self.order_ack_sent = False

    async def onLogon(self, sessionID, message=None):
        logger.info(f"[{sessionID}] Pyfixmsg_plus Acceptor: Received Logon from counterparty.")
        self.logon_received = True
        # Send a TestRequest (35=1) after logon
        if self.engine:
            test_req_id = f"test-{datetime.datetime.now().strftime('%H%M%S')}"
            test_request = self.engine.fixmsg({
                35: '1',
                34: self.engine.message_store.get_next_outgoing_sequence_number(),
                49: self.engine.sender,
                56: self.engine.target,
                112: test_req_id,
            })
            logger.info(f"[{sessionID}] Sending TestRequest: {test_request}")
            await self.engine.send_message(test_request)

    async def onMessage(self, message, sessionID):
        msg_type = message.get(35) if hasattr(message, 'get') else "Unknown"
        logger.info(f"[{sessionID}] Pyfixmsg_plus Acceptor: Received message type {msg_type}: {str(message)}")

        if msg_type == 'D' and not self.order_ack_sent:  # NewOrderSingle
            if not self.engine:
                logger.error(f"[{sessionID}] Engine not set in AcceptorApplication. Cannot send ExecutionReport.")
                return

            cl_ord_id = message.get(11)
            symbol = message.get(55)
            side = message.get(54)
            order_qty = message.get(38)
            price = message.get(44)

            if not all([cl_ord_id, symbol, side, order_qty]):
                logger.error(f"[{sessionID}] Missing required fields in NewOrderSingle to send ExecutionReport. ClOrdID={cl_ord_id}, Symbol={symbol}, Side={side}, OrderQty={order_qty}")
                return

            exec_report = self.engine.fixmsg({
                35: '8', 11: cl_ord_id, 37: f"exec-{cl_ord_id}", 17: f"execid-{cl_ord_id}",
                150: '0', 39: '0', 55: symbol, 54: side, 38: order_qty, 14: 0, 6: 0,
            })
            if price:
                exec_report[44] = price

            logger.info(f"[{sessionID}] Application: Sending ExecutionReport (ACK) for {cl_ord_id}")
            await self.engine.send_message(exec_report)
            self.order_ack_sent = True

async def main():
    logger.info("Starting acceptor engine...")
    script_dir = os.path.dirname(__file__)
    config_path = os.path.join(script_dir, 'config_acceptor.ini')
    
    config = ConfigManager(config_path)
    app = AcceptorApplication()
    engine = FixEngine(config, app) 
    
    # The FixEngine should call app.set_engine(self) during its initialization.
    # If it doesn't, this manual call ensures the app has the engine reference.
    if hasattr(app, 'set_engine'):
        app.set_engine(engine)

    try:
        await engine.start()
    except KeyboardInterrupt:
        logger.info("Acceptor engine shutting down by user request (KeyboardInterrupt)...")
    except Exception as e:
        logger.error(f"Error in acceptor: {e}", exc_info=True)
    finally:
        if engine and hasattr(engine, 'state_machine') and engine.state_machine.state.name != 'DISCONNECTED':
            logger.info("Ensuring engine is disconnected...")
            await engine.disconnect(graceful=True)
        logger.info("Acceptor main function finished.")

if __name__ == "__main__":
    script_dir_for_config = os.path.dirname(__file__)
    acceptor_config_file = os.path.join(script_dir_for_config, 'config_acceptor.ini')

    # Create a default config_acceptor.ini if it doesn't exist for the example
    if not os.path.exists(acceptor_config_file):
        # Use the ConfigManager instance with the correct path to create the file
        default_cfg_writer = ConfigManager(acceptor_config_file)
        default_cfg_writer.set('FIX', 'mode', 'acceptor')
        default_cfg_writer.set('FIX', 'sender', 'ACCEPTOR_SERVER') # Example: This acceptor's ID
        default_cfg_writer.set('FIX', 'target', 'INITIATOR_CLIENT') # Example: Expected client's ID
        default_cfg_writer.set('FIX', 'version', 'FIX.4.4')
        default_cfg_writer.set('FIX', 'spec_filename', 'FIX44.xml') # Ensure path is valid or in PYTHONPATH
        default_cfg_writer.set('FIX', 'host', '127.0.0.1')
        default_cfg_writer.set('FIX', 'port', '5000')
        default_cfg_writer.set('FIX', 'heartbeat_interval', '30')
        # retry_interval and max_retries are less relevant for acceptors
        default_cfg_writer.set('FIX', 'retry_interval', '5') 
        default_cfg_writer.set('FIX', 'max_retries', '0')  
        default_cfg_writer.set('FIX', 'state_file', os.path.join(script_dir_for_config, 'acceptor_fix_state.db'))
        default_cfg_writer.set('FIX', 'reset_seq_num_on_logon', 'false') 
        default_cfg_writer.save_config() # Use the correct save method name
        logger.info(f"Created default acceptor config: {acceptor_config_file}")
            
    # For cleaning up DB, read the path from the config file
    db_path_reader = ConfigManager(acceptor_config_file)
    db_file = db_path_reader.get('FIX', 'state_file', os.path.join(script_dir_for_config, 'acceptor_fix_state.db'))
    if os.path.exists(db_file):
        try:
            os.remove(db_file)
            logger.info(f"Removed previous acceptor state file: {db_file}")
        except OSError as e:
            logger.error(f"Error removing acceptor state file {db_file}: {e}")
            
    asyncio.run(main())
