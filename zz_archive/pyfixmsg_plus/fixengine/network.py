import asyncio
import ssl
import logging
from abc import ABC, abstractmethod

class NetworkConnection(ABC):
    def __init__(self, host, port, use_tls=False, certfile=None, keyfile=None):
        self.host = host
        self.port = port
        self.use_tls = use_tls
        self.certfile = certfile
        self.keyfile = keyfile
        self.reader = None
        self.writer = None
        self.logger = logging.getLogger(self.__class__.__name__)
        self.running = False
        self.buffer_size = 8192 # Default buffer size for reading

    def _create_ssl_context(self, server_side=False):
        context = ssl.create_default_context(ssl.Purpose.CLIENT_AUTH if server_side else ssl.Purpose.SERVER_AUTH)
        if server_side:
            if self.certfile and self.keyfile:
                context.load_cert_chain(certfile=self.certfile, keyfile=self.keyfile)
                self.logger.info(f"SSL context loaded with cert: {self.certfile}, key: {self.keyfile}")
            else:
                # This would typically raise an error or prevent TLS from being used
                self.logger.warning("TLS enabled for server, but certfile or keyfile not provided. Using default context (if any).")
        else: # Client side
            # For client, certfile might be a CA bundle if server uses self-signed certs, or not needed if server has a trusted cert.
            # keyfile is usually not needed for client unless doing mutual TLS.
            if self.certfile: # e.g. CA bundle
                 context.load_verify_locations(self.certfile)
                 self.logger.info(f"SSL context client side, loaded verify locations: {self.certfile}")
            context.check_hostname = True # Important for client to verify server hostname
            # Depending on server cert, might need: context.verify_mode = ssl.CERT_NONE for self-signed without proper CA
            # Or better: context.load_verify_locations(cafile='path/to/ca.pem')
        return context

    @abstractmethod
    async def connect(self):
        pass

    async def set_transport(self, reader, writer):
        """Sets the reader and writer, typically for an accepted connection."""
        self.reader = reader
        self.writer = writer
        self.logger.debug(f"Acceptor.set_transport: self.reader_id={id(self.reader)}, self.writer_id={id(self.writer)}")
        self.running = True
        peername = writer.get_extra_info('peername')
        self.logger.info(f"Transport set for {self.__class__.__name__}. Peer: {peername}")


    async def send(self, data: bytes):
        if self.writer and self.running:
            try:
                self.writer.write(data)
                await self.writer.drain()
                # self.logger.debug(f"Sent {len(data)} bytes: {data.decode(errors='replace')[:60]}...") # Too verbose for every send
            except ConnectionResetError:
                self.logger.error("Connection reset by peer during send.")
                await self.disconnect() # Ensure cleanup
                raise # Re-raise to allow caller to handle
            except Exception as e:
                self.logger.error(f"Error sending data: {e}", exc_info=True)
                await self.disconnect() # Ensure cleanup
                raise # Re-raise
        else:
            self.logger.warning("Cannot send data: writer is not available or not running.")
            # raise ConnectionError("Cannot send data: writer is not available or not running.")


    async def receive(self, handler):
        """Generic receive loop that passes data to a handler."""
        self.logger.debug(f"Acceptor.receive attempting with self.reader_id={id(self.reader) if self.reader else 'None'}")
        if not self.reader or not self.running:
            self.logger.warning("Cannot receive: reader is not available or not running.")
            return

        self.logger.info("Receive loop started.")
        try:
            while self.running:
                if self.reader.at_eof():
                    self.logger.info("EOF received, connection closed by peer.")
                    break
                data = await self.reader.read(self.buffer_size)
                
                # ****** Added diagnostic logging ******
                self.logger.debug(f"{self.__class__.__name__} network layer read {len(data)} bytes: {data[:100]}") # Log first 100 bytes
                # *************************************

                if not data:
                    self.logger.info("Connection closed by peer (EOF received, read returned no data).")
                    break 
                await handler(data) # Pass raw bytes to handler; handler decodes/parses
        except ConnectionResetError:
            self.logger.warning("Connection reset by peer during receive.")
        except asyncio.CancelledError:
             self.logger.info("Receive loop cancelled.")
        except Exception as e:
            self.logger.error(f"Error in receive loop: {e}", exc_info=True)
        finally:
            self.logger.info("Receive loop terminated.")
            # Ensure disconnect is called if loop terminates unexpectedly or due to EOF
            # This might be redundant if caller (FixEngine) also ensures disconnect,
            # but good for a clean network layer.
            if self.running: # If still marked as running, means an abnormal exit
                 self.logger.warning("Receive loop ended; ensuring final disconnect.")
                 await self.disconnect()


    async def disconnect(self):
        if not self.running and not self.writer: # Already disconnected or never connected
            self.logger.debug(f"{self.__class__.__name__} already disconnected or not connected.")
            return

        self.running = False
        if self.writer:
            try:
                if not self.writer.is_closing():
                    self.writer.close()
                    await self.writer.wait_closed()
                self.logger.info("Connection writer closed.")
            except Exception as e:
                self.logger.error(f"Error closing writer: {e}", exc_info=True)
            finally:
                self.writer = None
        self.reader = None # Clear reader as well
        self.logger.info(f"Disconnected from {self.host}:{self.port}.")


class Initiator(NetworkConnection):
    async def connect(self):
        if self.running:
            self.logger.info("Initiator already connected or connect attempt in progress.")
            return

        self.logger.info(f"Attempting to connect to {self.host}:{self.port} with TLS={self.use_tls}...")
        ssl_context = self._create_ssl_context(server_side=False) if self.use_tls else None
        try:
            self.reader, self.writer = await asyncio.open_connection(
                self.host, self.port, ssl=ssl_context
            )
            self.running = True
            self.logger.info(f"Successfully connected to {self.host}:{self.port}.")
        except ConnectionRefusedError:
            self.logger.error(f"Connection refused by {self.host}:{self.port}.")
            self.running = False # Ensure not marked as running
            raise # Re-raise for FixEngine to handle
        except ssl.SSLError as e:
            self.logger.error(f"SSL Error connecting to {self.host}:{self.port}: {e}", exc_info=True)
            self.running = False
            raise
        except Exception as e:
            self.logger.error(f"Failed to connect to {self.host}:{self.port}: {e}", exc_info=True)
            self.running = False
            raise


class Acceptor(NetworkConnection):
    def __init__(self, host, port, use_tls=False, certfile=None, keyfile=None):
        super().__init__(host, port, use_tls, certfile, keyfile)
        self.server = None # To hold the asyncio server object

    async def connect(self):
        # For Acceptor, 'connect' is more like 'start_listening'
        # The actual reader/writer for a specific client connection is handled by start_accepting's callback
        self.logger.warning("Acceptor.connect() called. Did you mean start_accepting(handler_coro_factory)?")
        # This method is not typically used for an acceptor in this model.
        # If it were to be used, it might imply a single client connection management,
        # which is not the typical role of the main Acceptor class.

    async def start_accepting(self, per_client_fix_engine_handler_coro_factory):
        """
        Starts listening for incoming connections.
        For each connection, it calls the provided `per_client_fix_engine_handler_coro_factory`
        which should be a coroutine factory that takes (reader, writer) and handles the FIX session.
        """
        if self.server is not None:
            self.logger.info("Acceptor already listening.")
            return

        ssl_context = self._create_ssl_context(server_side=True) if self.use_tls else None
        
        async def client_connected_cb(reader, writer):
            """Callback for each new client connection."""
            # This Acceptor instance itself doesn't directly use this reader/writer.
            # It delegates to a new handler (e.g., a new FixEngine instance or a dedicated session handler).
            # The per_client_fix_engine_handler_coro_factory is responsible for managing this specific client.
            self.logger.info(f"Client connected: {writer.get_extra_info('peername')}. Delegating to handler factory.")
            try:
                # The factory should create and run the handler for the client session
                await per_client_fix_engine_handler_coro_factory(reader, writer)
            except Exception as e:
                self.logger.error(f"Unhandled exception in client_connected_cb for {writer.get_extra_info('peername')}: {e}", exc_info=True)
            finally:
                # Ensure client connection is closed if handler didn't do it or crashed
                if writer and not writer.is_closing():
                    self.logger.warning(f"Handler for {writer.get_extra_info('peername')} exited; ensuring writer is closed.")
                    writer.close()
                    try:
                        await writer.wait_closed()
                    except Exception: 
                        pass # Ignore errors on final close attempt

        try:
            self.server = await asyncio.start_server(
                client_connected_cb, self.host, self.port, ssl=ssl_context
            )
            addrs = ', '.join(str(sock.getsockname()) for sock in self.server.sockets)
            self.logger.info(f"Acceptor listening on {addrs} with TLS={self.use_tls}")
            self.running = True # Mark as running (listening)
            async with self.server:
                await self.server.serve_forever()
        except asyncio.CancelledError:
            self.logger.info("Acceptor serve_forever() cancelled.")
        except Exception as e:
            self.logger.error(f"Failed to start acceptor: {e}", exc_info=True)
            self.running = False
        finally:
            self.logger.info("Acceptor has stopped listening.")
            self.running = False
            if self.server:
                self.server.close()
                await self.server.wait_closed()
                self.server = None
    
    async def disconnect(self):
        """Stops the acceptor server from listening for new connections."""
        self.running = False # Stop accepting new connections
        if self.server:
            self.logger.info("Closing acceptor server...")
            self.server.close()
            await self.server.wait_closed()
            self.server = None
            self.logger.info("Acceptor server closed.")
        else:
            self.logger.info("Acceptor server was not running or already closed.")
        # Note: This stops new connections. Existing client connections
        # are managed by their respective handlers (e.g., FixEngine instances spawned by the factory).
        # The generic NetworkConnection.disconnect() is for a single reader/writer pair,
        # which the main Acceptor class doesn't hold directly (it delegates).
        # So, we override to control the listening server.


    async def set_transport(self, reader, writer):
        """
        For an Acceptor that handles a single client connection (not the typical server model),
        this method would set its reader/writer.
        However, in the asyncio.start_server model, the main Acceptor delegates
        reader/writer pairs to a callback. If this Acceptor instance is *also*
        going to handle one specific client (e.g., after being passed reader/writer
        from the callback), then this method is appropriate.
        """
        await super().set_transport(reader, writer)
        # If this Acceptor instance is now dedicated to this client,
        # its receive loop would use self.reader and self.writer.
        # This is what happens if FixEngine is used as the client_connected_cb directly.
