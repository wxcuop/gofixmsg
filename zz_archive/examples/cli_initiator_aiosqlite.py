import os
import asyncio
import logging
from pyfixmsg_plus.fixengine.configmanager import ConfigManager
from pyfixmsg_plus.fixengine.engine import FixEngine
from pyfixmsg_plus.idgen.id_generator import YMDClOrdIdGenerator
from sample_application import DummyApplication, run_common_initiator_logic

# Basic logging setup for the example
logging.basicConfig(
    level=logging.DEBUG,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
)
logger = logging.getLogger(__name__)

async def main(config_manager):
    app = DummyApplication()
    engine = await FixEngine.create(config_manager, app)
    if hasattr(app, 'set_engine'):
        app.set_engine(engine)
    if hasattr(engine, "initialize") and callable(engine.initialize):
        await engine.initialize()

    clordid_generator = YMDClOrdIdGenerator()
    engine_task = None
    try:
        logger.info(f"Starting initiator engine (Sender: {engine.sender}, Target: {engine.target}) to connect to {engine.host}:{engine.port}...")
        engine_task = asyncio.create_task(engine.start())
        logger.info("Initiator engine.start() task created. Allowing time for connection attempt...")
        await asyncio.sleep(2)
        await run_common_initiator_logic(engine, clordid_generator, logger, max_loops_disconnected=5)
        logger.info(f"Initiator example loop finished. Final engine state: {engine.state_machine.state.name if engine and hasattr(engine, 'state_machine') else 'UNKNOWN'}")
    except KeyboardInterrupt:
        logger.info("Initiator shutting down due to KeyboardInterrupt...")
    except ConnectionRefusedError:
        logger.error(f"Connection refused when trying to connect to {engine.host if engine else 'UNKNOWN_HOST'}:{engine.port if engine else 'UNKNOWN_PORT'}. Ensure acceptor is running.")
    except Exception as e:
        logger.error(f"Error in initiator: {e}", exc_info=True)
    finally:
        if engine_task and not engine_task.done():
            logger.info("Cancelling engine task...")
            engine_task.cancel()
            try:
                await engine_task
            except asyncio.CancelledError:
                logger.info("Engine task successfully cancelled.")
            except Exception as e_task_cancel:
                logger.error(f"Exception while cancelling engine task: {e_task_cancel}", exc_info=True)

        if engine and hasattr(engine, 'state_machine') and engine.state_machine.state.name != 'DISCONNECTED':
            logger.info("Ensuring initiator engine is disconnected (final check)...")
            await engine.disconnect(graceful=False)
        logger.info("Initiator main function finished.")

if __name__ == "__main__":
    script_dir = os.path.dirname(__file__)
    initiator_config_file = os.path.join(script_dir, 'config_initiator_aiosqlite.ini')

    if not os.path.exists(initiator_config_file):
        default_cfg = ConfigManager(initiator_config_file)
        default_cfg.set('FIX', 'mode', 'initiator')
        default_cfg.set('FIX', 'sender', 'BANZAI')
        default_cfg.set('FIX', 'target', 'EXEC')
        default_cfg.set('FIX', 'version', 'FIX.4.4')
        default_cfg.set('FIX', 'spec_filename', 'FIX44.xml')
        default_cfg.set('FIX', 'host', '127.0.0.1')
        default_cfg.set('FIX', 'port', '9881')
        default_cfg.set('FIX', 'heartbeat_interval', '30')
        default_cfg.set('FIX', 'retry_interval', '5')
        default_cfg.set('FIX', 'max_retries', '3')
        default_cfg.set('FIX', 'state_file', os.path.join(script_dir, 'initiator_fix_state_aiosqlite.db'))
        default_cfg.set('FIX', 'message_store_type', 'aiosqlite')
        default_cfg.set('FIX', 'reset_seq_num_on_logon', 'true')
        default_cfg.set('FIX', 'EncryptMethod', '0')
        default_cfg.save_config()
        logger.info(f"Created default aiosqlite initiator config: {initiator_config_file}")

    config_manager = ConfigManager(initiator_config_file)
    db_file = config_manager.get('FIX', 'state_file', os.path.join(script_dir, 'initiator_fix_state_aiosqlite.db'))
    if os.path.exists(db_file):
        try:
            os.remove(db_file)
            logger.info(f"Removed previous aiosqlite initiator state file: {db_file}")
        except OSError as e:
            logger.error(f"Error removing aiosqlite initiator state file {db_file}: {e}")

    asyncio.run(main(config_manager))