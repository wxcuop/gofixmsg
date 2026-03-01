"""
Performance tests for PyFixMsg Plus FIX Engine.
Tests throughput, latency, memory usage, and scalability under load.
"""
import asyncio
import time
import psutil
import gc
import statistics
from collections import defaultdict
import pytest
from concurrent.futures import ThreadPoolExecutor
from unittest.mock import Mock

from pyfixmsg_plus.fixengine.engine import FixEngine
from pyfixmsg_plus.application import Application


@pytest.mark.performance
@pytest.mark.slow
@pytest.mark.asyncio
class TestThroughputPerformance:
    """Test message processing throughput under various loads."""
    
    async def test_single_session_throughput(self, fix_engine_pair, mock_application, performance_metrics):
        """Test maximum throughput for a single session."""
        initiator, acceptor = fix_engine_pair
        
        # Track received messages
        received_count = 0
        start_time = None
        
        async def count_received_messages(session_id, message):
            nonlocal received_count, start_time
            if start_time is None:
                start_time = time.time()
            received_count += 1
        
        mock_application.from_app = count_received_messages
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start session
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        await asyncio.sleep(1.0)
        
        assert initiator.is_logged_on()
        
        # Performance test parameters
        message_count = 10000
        batch_size = 100
        
        # Start timing
        test_start = time.time()
        
        # Send messages in batches for better performance
        for batch in range(0, message_count, batch_size):
            tasks = []
            for i in range(batch_size):
                if batch + i >= message_count:
                    break
                
                message = {
                    '8': 'FIX.4.4',
                    '35': 'D',  # NewOrderSingle
                    '49': 'INITIATOR',
                    '56': 'ACCEPTOR',
                    '34': str(batch + i + 2),
                    '52': '20250726-12:00:00.000',
                    '11': f'ORDER{batch + i}',
                    '21': '1',
                    '38': '100',
                    '40': '2',
                    '44': '50.25',
                    '54': '1',
                    '55': 'MSFT',
                    '59': '0',
                }
                
                tasks.append(initiator.send_to_target(message))
            
            # Send batch
            await asyncio.gather(*tasks)
            
            # Small delay to prevent overwhelming
            if batch % (batch_size * 10) == 0:
                await asyncio.sleep(0.01)
        
        # Wait for all messages to be processed
        timeout = 30.0  # 30 second timeout
        end_time = time.time() + timeout
        
        while received_count < message_count and time.time() < end_time:
            await asyncio.sleep(0.1)
        
        test_end = time.time()
        test_duration = test_end - test_start
        
        # Calculate metrics
        throughput = message_count / test_duration
        
        print(f"\nThroughput Test Results:")
        print(f"Messages sent: {message_count}")
        print(f"Messages received: {received_count}")
        print(f"Test duration: {test_duration:.2f} seconds")
        print(f"Throughput: {throughput:.2f} messages/second")
        
        # Performance assertions (adjust based on requirements)
        assert received_count >= message_count * 0.95  # Allow 5% message loss
        assert throughput >= 1000  # Minimum 1000 msg/sec
        
        # Store metrics
        performance_metrics['throughput'] = throughput
        performance_metrics['message_count'] = message_count
        performance_metrics['received_count'] = received_count
        performance_metrics['test_duration'] = test_duration
        
        # Cleanup
        await initiator.stop()
        await acceptor.stop()
    
    async def test_concurrent_sessions_throughput(self, sample_config_dict, free_port):
        """Test throughput with multiple concurrent sessions."""
        session_count = 5
        messages_per_session = 1000
        
        # Setup multiple sessions
        sessions = []
        received_counts = defaultdict(int)
        
        # Create acceptor
        acceptor_config = sample_config_dict.copy()
        acceptor_config['session']['sender_comp_id'] = 'ACCEPTOR'
        acceptor_config['network']['socket_accept_port'] = free_port
        
        import tempfile
        import os
        
        with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
            for section, options in acceptor_config.items():
                f.write(f'[{section}]\n')
                for key, value in options.items():
                    f.write(f'{key} = {value}\n')
                f.write('\n')
            acceptor_config_path = f.name
        
        from pyfixmsg_plus.fixengine.configmanager import ConfigManager
        acceptor_cm = ConfigManager(acceptor_config_path)
        acceptor = FixEngine(acceptor_cm)
        
        async def count_messages(session_id, message):
            received_counts[session_id] += 1
        
        acceptor_app = Mock(spec=Application)
        acceptor_app.from_app = count_messages
        acceptor.application = acceptor_app
        
        await acceptor.start()
        await asyncio.sleep(0.1)
        
        try:
            # Create and start multiple initiators
            for i in range(session_count):
                config = sample_config_dict.copy()
                config['session']['sender_comp_id'] = f'INIT{i}'
                config['session']['target_comp_id'] = 'ACCEPTOR'
                config['network']['port'] = free_port
                
                with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
                    for section, options in config.items():
                        f.write(f'[{section}]\n')
                        for key, value in options.items():
                            f.write(f'{key} = {value}\n')
                        f.write('\n')
                    config_path = f.name
                
                cm = ConfigManager(config_path)
                initiator = FixEngine(cm)
                initiator.application = Mock(spec=Application)
                sessions.append((initiator, config_path))
                
                await initiator.start()
                await asyncio.sleep(0.1)
            
            # Wait for all sessions to establish
            await asyncio.sleep(2.0)
            
            # Verify all sessions connected
            for initiator, _ in sessions:
                assert initiator.is_logged_on()
            
            # Start performance test
            start_time = time.time()
            
            # Send messages from all sessions concurrently
            async def send_session_messages(session_id, initiator):
                for i in range(messages_per_session):
                    message = {
                        '8': 'FIX.4.4',
                        '35': '0',  # Heartbeat for simplicity
                        '49': f'INIT{session_id}',
                        '56': 'ACCEPTOR',
                        '34': str(i + 2),
                        '52': '20250726-12:00:00.000',
                    }
                    await initiator.send_to_target(message)
                    
                    # Small delay to control rate
                    if i % 100 == 0:
                        await asyncio.sleep(0.01)
            
            # Start all sessions sending concurrently
            tasks = []
            for i, (initiator, _) in enumerate(sessions):
                tasks.append(send_session_messages(i, initiator))
            
            await asyncio.gather(*tasks)
            
            # Wait for message processing
            await asyncio.sleep(5.0)
            
            end_time = time.time()
            test_duration = end_time - start_time
            
            # Calculate results
            total_sent = session_count * messages_per_session
            total_received = sum(received_counts.values())
            overall_throughput = total_sent / test_duration
            
            print(f"\nConcurrent Sessions Throughput Test:")
            print(f"Sessions: {session_count}")
            print(f"Messages per session: {messages_per_session}")
            print(f"Total messages sent: {total_sent}")
            print(f"Total messages received: {total_received}")
            print(f"Test duration: {test_duration:.2f} seconds")
            print(f"Overall throughput: {overall_throughput:.2f} messages/second")
            print(f"Per-session throughput: {overall_throughput/session_count:.2f} messages/second")
            
            # Performance assertions
            assert total_received >= total_sent * 0.9  # Allow 10% loss for concurrent test
            assert overall_throughput >= 2000  # Minimum total throughput
            
        finally:
            # Cleanup
            for initiator, config_path in sessions:
                if initiator._running:
                    await initiator.stop()
                os.unlink(config_path)
            
            await acceptor.stop()
            os.unlink(acceptor_config_path)


@pytest.mark.performance
@pytest.mark.slow
@pytest.mark.asyncio
class TestLatencyPerformance:
    """Test message processing latency and response times."""
    
    async def test_message_processing_latency(self, fix_engine_pair, mock_application):
        """Test end-to-end message processing latency."""
        initiator, acceptor = fix_engine_pair
        
        # Track message latencies
        latencies = []
        message_timestamps = {}
        
        async def record_message_latency(session_id, message):
            msg_id = message.get('11', message.get('34'))  # ClOrdID or SeqNum
            if msg_id in message_timestamps:
                receive_time = time.time()
                send_time = message_timestamps[msg_id]
                latency = (receive_time - send_time) * 1000  # Convert to milliseconds
                latencies.append(latency)
        
        mock_application.from_app = record_message_latency
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start session
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        await asyncio.sleep(1.0)
        
        assert initiator.is_logged_on()
        
        # Send test messages with timing
        message_count = 1000
        
        for i in range(message_count):
            msg_id = f'ORDER{i}'
            message = {
                '8': 'FIX.4.4',
                '35': 'D',  # NewOrderSingle
                '49': 'INITIATOR',
                '56': 'ACCEPTOR',
                '34': str(i + 2),
                '52': '20250726-12:00:00.000',
                '11': msg_id,
                '21': '1',
                '38': '100',
                '40': '2',
                '44': '50.25',
                '54': '1',
                '55': 'MSFT',
                '59': '0',
            }
            
            # Record send time
            message_timestamps[msg_id] = time.time()
            await initiator.send_to_target(message)
            
            # Control send rate
            if i % 100 == 0:
                await asyncio.sleep(0.1)
        
        # Wait for all messages to be processed
        timeout = 30.0
        end_time = time.time() + timeout
        
        while len(latencies) < message_count * 0.95 and time.time() < end_time:
            await asyncio.sleep(0.1)
        
        # Calculate latency statistics
        if latencies:
            avg_latency = statistics.mean(latencies)
            median_latency = statistics.median(latencies)
            p95_latency = sorted(latencies)[int(len(latencies) * 0.95)]
            p99_latency = sorted(latencies)[int(len(latencies) * 0.99)]
            max_latency = max(latencies)
            min_latency = min(latencies)
            
            print(f"\nLatency Test Results:")
            print(f"Messages processed: {len(latencies)}")
            print(f"Average latency: {avg_latency:.2f} ms")
            print(f"Median latency: {median_latency:.2f} ms")
            print(f"95th percentile: {p95_latency:.2f} ms")
            print(f"99th percentile: {p99_latency:.2f} ms")
            print(f"Min latency: {min_latency:.2f} ms")
            print(f"Max latency: {max_latency:.2f} ms")
            
            # Performance assertions (adjust based on requirements)
            assert avg_latency < 5.0  # Average < 5ms
            assert p95_latency < 10.0  # 95th percentile < 10ms
            assert p99_latency < 20.0  # 99th percentile < 20ms
        
        # Cleanup
        await initiator.stop()
        await acceptor.stop()


@pytest.mark.performance
@pytest.mark.slow
@pytest.mark.asyncio
class TestMemoryPerformance:
    """Test memory usage and leak detection."""
    
    async def test_memory_usage_during_load(self, fix_engine_pair, mock_application):
        """Test memory usage during sustained load."""
        initiator, acceptor = fix_engine_pair
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start session
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        await asyncio.sleep(1.0)
        
        assert initiator.is_logged_on()
        
        # Record initial memory usage
        process = psutil.Process()
        initial_memory = process.memory_info().rss / 1024 / 1024  # MB
        
        print(f"\nInitial memory usage: {initial_memory:.2f} MB")
        
        # Run sustained load test
        message_count = 5000
        memory_samples = []
        
        for i in range(message_count):
            message = {
                '8': 'FIX.4.4',
                '35': '0',  # Heartbeat
                '49': 'INITIATOR',
                '56': 'ACCEPTOR',
                '34': str(i + 2),
                '52': '20250726-12:00:00.000',
            }
            
            await initiator.send_to_target(message)
            
            # Sample memory usage periodically
            if i % 500 == 0:
                current_memory = process.memory_info().rss / 1024 / 1024  # MB
                memory_samples.append(current_memory)
                
                # Force garbage collection
                gc.collect()
            
            # Small delay to control rate
            if i % 100 == 0:
                await asyncio.sleep(0.01)
        
        # Wait for processing to complete
        await asyncio.sleep(2.0)
        
        # Final memory measurement
        final_memory = process.memory_info().rss / 1024 / 1024  # MB
        
        print(f"Final memory usage: {final_memory:.2f} MB")
        print(f"Memory increase: {final_memory - initial_memory:.2f} MB")
        print(f"Memory samples: {memory_samples}")
        
        # Memory usage assertions
        memory_increase = final_memory - initial_memory
        assert memory_increase < 100  # Should not increase by more than 100MB
        
        # Check for memory leaks (no sustained growth)
        if len(memory_samples) >= 3:
            trend = memory_samples[-1] - memory_samples[0]
            assert trend < 50  # No more than 50MB growth during test
        
        # Cleanup
        await initiator.stop()
        await acceptor.stop()
    
    async def test_memory_leak_detection(self, config_manager, mock_application):
        """Test for memory leaks over multiple session cycles."""
        memory_readings = []
        process = psutil.Process()
        
        # Run multiple session creation/destruction cycles
        cycles = 5
        
        for cycle in range(cycles):
            # Record memory before cycle
            gc.collect()  # Force garbage collection
            memory_before = process.memory_info().rss / 1024 / 1024  # MB
            
            # Create and run session
            engine = FixEngine(config_manager)
            engine.application = mock_application
            
            await engine.start()
            await asyncio.sleep(1.0)
            
            # Send some messages
            for i in range(100):
                message = {
                    '8': 'FIX.4.4',
                    '35': '0',  # Heartbeat
                    '49': 'SENDER',
                    '56': 'TARGET',
                    '34': str(i + 1),
                    '52': '20250726-12:00:00.000',
                }
                # Note: This might fail if session isn't established
                try:
                    await engine.send_to_target(message)
                except:
                    pass
            
            await asyncio.sleep(0.5)
            
            # Stop session
            await engine.stop()
            del engine
            
            # Record memory after cycle
            gc.collect()  # Force garbage collection
            memory_after = process.memory_info().rss / 1024 / 1024  # MB
            
            memory_readings.append({
                'cycle': cycle,
                'before': memory_before,
                'after': memory_after,
                'difference': memory_after - memory_before
            })
            
            print(f"Cycle {cycle}: {memory_before:.2f} MB -> {memory_after:.2f} MB "
                  f"(+{memory_after - memory_before:.2f} MB)")
        
        # Analyze memory leak
        if len(memory_readings) >= 3:
            first_cycle_memory = memory_readings[0]['after']
            last_cycle_memory = memory_readings[-1]['after']
            total_growth = last_cycle_memory - first_cycle_memory
            
            print(f"\nMemory leak analysis:")
            print(f"First cycle final memory: {first_cycle_memory:.2f} MB")
            print(f"Last cycle final memory: {last_cycle_memory:.2f} MB")
            print(f"Total memory growth: {total_growth:.2f} MB")
            
            # Assert no significant memory leak
            assert total_growth < 20  # Less than 20MB growth over all cycles


@pytest.mark.performance
@pytest.mark.slow
class TestCPUPerformance:
    """Test CPU usage under load."""
    
    @pytest.mark.asyncio
    async def test_cpu_usage_under_load(self, fix_engine_pair, mock_application):
        """Test CPU usage during high message throughput."""
        initiator, acceptor = fix_engine_pair
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start session
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        await asyncio.sleep(1.0)
        
        assert initiator.is_logged_on()
        
        # Start CPU monitoring
        process = psutil.Process()
        cpu_samples = []
        
        async def monitor_cpu():
            """Monitor CPU usage during test."""
            for _ in range(30):  # 30 seconds of monitoring
                cpu_percent = process.cpu_percent()
                cpu_samples.append(cpu_percent)
                await asyncio.sleep(1.0)
        
        # Start CPU monitoring task
        monitor_task = asyncio.create_task(monitor_cpu())
        
        # Generate load
        message_count = 5000
        
        for i in range(message_count):
            message = {
                '8': 'FIX.4.4',
                '35': 'D',  # NewOrderSingle
                '49': 'INITIATOR',
                '56': 'ACCEPTOR',
                '34': str(i + 2),
                '52': '20250726-12:00:00.000',
                '11': f'ORDER{i}',
                '21': '1',
                '38': '100',
                '40': '2',
                '44': '50.25',
                '54': '1',
                '55': 'MSFT',
                '59': '0',
            }
            
            await initiator.send_to_target(message)
            
            # Control message rate
            if i % 200 == 0:
                await asyncio.sleep(0.01)
        
        # Wait for monitoring to complete
        await asyncio.sleep(5.0)
        monitor_task.cancel()
        
        # Analyze CPU usage
        if cpu_samples:
            avg_cpu = statistics.mean(cpu_samples)
            max_cpu = max(cpu_samples)
            
            print(f"\nCPU Performance Results:")
            print(f"Average CPU usage: {avg_cpu:.2f}%")
            print(f"Peak CPU usage: {max_cpu:.2f}%")
            print(f"CPU samples: {len(cpu_samples)}")
            
            # CPU usage assertions (adjust based on requirements)
            assert avg_cpu < 50.0  # Average CPU < 50%
            assert max_cpu < 80.0  # Peak CPU < 80%
        
        # Cleanup
        await initiator.stop()
        await acceptor.stop()


if __name__ == '__main__':
    pytest.main([__file__, '-v', '--tb=short', '-m', 'performance'])
