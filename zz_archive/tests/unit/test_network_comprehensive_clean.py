"""
Comprehensive unit tests for Network: connection lifecycle, error handling, async operations, message routing.
"""
import asyncio
import pytest
from unittest.mock import Mock, AsyncMock, patch
from pyfixmsg_plus.fixengine.network import NetworkConnection, Initiator, Acceptor

class TestNetworkConnectionInitiator:
    def test_initiator_creation(self):
        initiator = Initiator(host='localhost', port=8080)
        assert initiator.host == 'localhost'
        assert initiator.port == 8080
        assert initiator.use_tls is False
        assert initiator.running is False

    def test_initiator_tls_creation(self):
        initiator = Initiator(host='localhost', port=8080, use_tls=True, certfile='cert.pem', keyfile='key.pem')
        assert initiator.use_tls is True
        assert initiator.certfile == 'cert.pem'
        assert initiator.keyfile == 'key.pem'

    @pytest.mark.asyncio
    async def test_initiator_connect_success(self):
        initiator = Initiator(host='localhost', port=8080)
        
        with patch('asyncio.open_connection') as mock_open:
            mock_reader = AsyncMock()
            mock_writer = AsyncMock()
            mock_open.return_value = (mock_reader, mock_writer)
            
            await initiator.connect()
            
            assert initiator.running is True
            assert initiator.reader == mock_reader
            assert initiator.writer == mock_writer
            mock_open.assert_called_once_with('localhost', 8080, ssl=None)

    @pytest.mark.asyncio
    async def test_initiator_connect_failure(self):
        initiator = Initiator(host='localhost', port=8080)
        
        with patch('asyncio.open_connection') as mock_open:
            mock_open.side_effect = ConnectionRefusedError("Connection refused")
            
            with pytest.raises(ConnectionRefusedError):
                await initiator.connect()
            
            assert initiator.running is False

    @pytest.mark.asyncio
    async def test_initiator_send_message(self):
        initiator = Initiator(host='localhost', port=8080)
        mock_writer = AsyncMock()
        initiator.writer = mock_writer
        initiator.running = True
        
        test_data = b"8=FIX.4.4|9=100|35=D|..."
        await initiator.send(test_data)
        
        mock_writer.write.assert_called_once_with(test_data)
        mock_writer.drain.assert_called_once()

    @pytest.mark.asyncio
    async def test_initiator_disconnect(self):
        initiator = Initiator(host='localhost', port=8080)
        mock_writer = AsyncMock()
        initiator.writer = mock_writer
        initiator.running = True
        
        await initiator.disconnect()
        
        assert initiator.running is False
        assert initiator.writer is None
        mock_writer.close.assert_called_once()

class TestNetworkConnectionAcceptor:
    def test_acceptor_creation(self):
        acceptor = Acceptor(host='0.0.0.0', port=8080)
        assert acceptor.host == '0.0.0.0'
        assert acceptor.port == 8080
        assert acceptor.use_tls is False
        assert acceptor.running is False
        assert acceptor.server is None

    def test_acceptor_tls_creation(self):
        acceptor = Acceptor(host='0.0.0.0', port=8080, use_tls=True, certfile='cert.pem', keyfile='key.pem')
        assert acceptor.use_tls is True
        assert acceptor.certfile == 'cert.pem'
        assert acceptor.keyfile == 'key.pem'

    @pytest.mark.asyncio
    async def test_acceptor_start_accepting(self):
        acceptor = Acceptor(host='0.0.0.0', port=8080)
        mock_handler = AsyncMock()
        
        with patch('asyncio.start_server') as mock_start_server:
            mock_server = AsyncMock()
            mock_server.sockets = [Mock()]
            mock_server.sockets[0].getsockname.return_value = ('0.0.0.0', 8080)
            mock_start_server.return_value = mock_server
            
            # Use asyncio.CancelledError to exit the serve_forever loop
            mock_server.serve_forever.side_effect = asyncio.CancelledError()
            
            try:
                await acceptor.start_accepting(mock_handler)
            except asyncio.CancelledError:
                pass  # Expected to exit this way in test
            
            mock_start_server.assert_called_once()
            assert acceptor.running is False  # Should be False after cancellation

    @pytest.mark.asyncio
    async def test_acceptor_disconnect(self):
        acceptor = Acceptor(host='0.0.0.0', port=8080)
        mock_server = AsyncMock()
        acceptor.server = mock_server
        acceptor.running = True
        
        await acceptor.disconnect()
        
        assert acceptor.running is False
        assert acceptor.server is None
        mock_server.close.assert_called_once()

    @pytest.mark.asyncio
    async def test_acceptor_set_transport(self):
        acceptor = Acceptor(host='0.0.0.0', port=8080)
        mock_reader = AsyncMock()
        mock_writer = AsyncMock()
        mock_writer.get_extra_info.return_value = ('127.0.0.1', 12345)
        
        await acceptor.set_transport(mock_reader, mock_writer)
        
        assert acceptor.reader == mock_reader
        assert acceptor.writer == mock_writer
        assert acceptor.running is True

class TestNetworkConnectionBase:
    @pytest.mark.asyncio
    async def test_send_without_writer(self):
        initiator = Initiator(host='localhost', port=8080)
        # Don't set writer, running is False
        
        test_data = b"test data"
        # Should not raise exception, just log warning
        await initiator.send(test_data)

    @pytest.mark.asyncio
    async def test_send_connection_reset(self):
        initiator = Initiator(host='localhost', port=8080)
        mock_writer = AsyncMock()
        mock_writer.write.side_effect = ConnectionResetError("Connection reset")
        initiator.writer = mock_writer
        initiator.running = True
        
        with pytest.raises(ConnectionResetError):
            await initiator.send(b"test data")
        
        # Should trigger disconnect
        assert initiator.running is False

    @pytest.mark.asyncio
    async def test_receive_with_handler(self):
        initiator = Initiator(host='localhost', port=8080)
        mock_reader = AsyncMock()
        mock_handler = AsyncMock()
        
        # Simulate receiving some data then EOF
        mock_reader.at_eof.side_effect = [False, True]
        mock_reader.read.return_value = b"test data"
        
        initiator.reader = mock_reader
        initiator.running = True
        
        await initiator.receive(mock_handler)
        
        mock_handler.assert_called_once_with(b"test data")
        assert initiator.running is False  # Should disconnect after EOF

    @pytest.mark.asyncio
    async def test_receive_without_reader(self):
        initiator = Initiator(host='localhost', port=8080)
        mock_handler = AsyncMock()
        
        # Should return immediately if no reader
        await initiator.receive(mock_handler)
        mock_handler.assert_not_called()

    def test_ssl_context_creation_client(self):
        initiator = Initiator(host='localhost', port=8080, use_tls=True, certfile='ca.pem')
        
        with patch('ssl.create_default_context') as mock_create_context:
            mock_context = Mock()
            mock_create_context.return_value = mock_context
            
            context = initiator._create_ssl_context(server_side=False)
            
            assert context == mock_context
            mock_context.load_verify_locations.assert_called_once_with('ca.pem')

    def test_ssl_context_creation_server(self):
        acceptor = Acceptor(host='0.0.0.0', port=8080, use_tls=True, certfile='cert.pem', keyfile='key.pem')
        
        with patch('ssl.create_default_context') as mock_create_context:
            mock_context = Mock()
            mock_create_context.return_value = mock_context
            
            context = acceptor._create_ssl_context(server_side=True)
            
            assert context == mock_context
            mock_context.load_cert_chain.assert_called_once_with(certfile='cert.pem', keyfile='key.pem')

class TestNetworkConnectionPropertyBased:
    def test_all_connection_types_have_required_attributes(self):
        """Test that all network connection types have required attributes"""
        initiator = Initiator(host='localhost', port=8080)
        acceptor = Acceptor(host='0.0.0.0', port=8080)
        
        for conn in [initiator, acceptor]:
            assert hasattr(conn, 'host')
            assert hasattr(conn, 'port')
            assert hasattr(conn, 'use_tls')
            assert hasattr(conn, 'running')
            assert hasattr(conn, 'reader')
            assert hasattr(conn, 'writer')
            assert hasattr(conn, 'logger')

    def test_buffer_size_consistency(self):
        """Test that buffer size is consistent across connection types"""
        initiator = Initiator(host='localhost', port=8080)
        acceptor = Acceptor(host='0.0.0.0', port=8080)
        
        assert initiator.buffer_size == acceptor.buffer_size == 8192

    @pytest.mark.asyncio
    async def test_disconnect_idempotency(self):
        """Test that disconnect can be called multiple times safely"""
        initiator = Initiator(host='localhost', port=8080)
        
        # First disconnect
        await initiator.disconnect()
        assert initiator.running is False
        
        # Second disconnect should not raise exception
        await initiator.disconnect()
        assert initiator.running is False
