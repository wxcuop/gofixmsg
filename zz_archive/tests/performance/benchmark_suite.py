"""
Memory profiling and benchmarking tests for PyFixMsg Plus.
Detailed performance analysis and memory usage tracking.
"""
import asyncio
import time
import gc
import psutil
import memory_profiler
import tracemalloc
import statistics
from contextlib import contextmanager
import pytest
from unittest.mock import Mock

from pyfixmsg_plus.fixengine.engine import FixEngine
from pyfixmsg_plus.application import Application


@contextmanager
def memory_monitor():
    """Context manager for monitoring memory usage."""
    process = psutil.Process()
    
    # Start memory tracking
    tracemalloc.start()
    gc.collect()
    
    initial_memory = process.memory_info().rss / 1024 / 1024  # MB
    initial_snapshot = tracemalloc.take_snapshot()
    
    yield {
        'initial_memory': initial_memory,
        'process': process
    }
    
    # Final measurements
    gc.collect()
    final_memory = process.memory_info().rss / 1024 / 1024  # MB
    final_snapshot = tracemalloc.take_snapshot()
    
    # Calculate differences
    memory_diff = final_memory - initial_memory
    
    # Top memory allocations
    top_stats = final_snapshot.compare_to(initial_snapshot, 'lineno')
    
    print(f"\n{'='*60}")
    print(f"MEMORY ANALYSIS")
    print(f"{'='*60}")
    print(f"Initial memory: {initial_memory:.2f} MB")
    print(f"Final memory: {final_memory:.2f} MB")
    print(f"Memory difference: {memory_diff:.2f} MB")
    print(f"\nTop 10 memory allocations:")
    
    for stat in top_stats[:10]:
        print(f"  {stat}")
    
    tracemalloc.stop()


@pytest.mark.performance
@pytest.mark.slow
class TestMemoryProfiling:
    """Memory profiling tests for various scenarios."""
    
    @pytest.mark.asyncio
    async def test_session_creation_memory_usage(self, fix_engine, mock_application):
        """Profile memory usage during session creation."""
        
        with memory_monitor() as monitor:
            fix_engine.application = mock_application
            
            # Create multiple sessions (simulated)
            for i in range(10):
                try:
                    await fix_engine.start()
                    await asyncio.sleep(0.1)
                    await fix_engine.stop()
                    await asyncio.sleep(0.1)
                    
                    # Force garbage collection
                    gc.collect()
                    
                    # Sample memory every few iterations
                    if i % 3 == 0:
                        current_memory = monitor['process'].memory_info().rss / 1024 / 1024
                        print(f"  Iteration {i}: {current_memory:.2f} MB")
                        
                except Exception as e:
                    # Session creation might fail in test environment
                    print(f"  Session {i} failed: {e}")
    
    @pytest.mark.asyncio 
    async def test_message_processing_memory_growth(self, fix_engine, mock_application):
        """Profile memory growth during message processing."""
        
        # Track received messages
        received_messages = []
        
        async def track_messages(session_id, message):
            received_messages.append(message)
        
        mock_application.from_app = track_messages
        fix_engine.application = mock_application
        
        with memory_monitor() as monitor:
            try:
                await fix_engine.start()
                await asyncio.sleep(0.5)
                
                # Send many messages to test memory usage
                message_count = 1000
                memory_samples = []
                
                for i in range(message_count):
                    message = {
                        '8': 'FIX.4.4',
                        '35': '0',  # Heartbeat
                        '49': 'SENDER',
                        '56': 'TARGET',
                        '34': str(i + 1),
                        '52': '20250726-12:00:00.000',
                        # Add some payload to test memory usage
                        '58': f'Test message {i} with some payload data' * 10,
                    }
                    
                    try:
                        await fix_engine.send_to_target(message)
                    except Exception:
                        # Sending might fail if not connected
                        pass
                    
                    # Sample memory every 100 messages
                    if i % 100 == 0:
                        current_memory = monitor['process'].memory_info().rss / 1024 / 1024
                        memory_samples.append(current_memory)
                        print(f"  Message {i}: {current_memory:.2f} MB")
                    
                    # Small delay
                    if i % 50 == 0:
                        await asyncio.sleep(0.01)
                
                # Analyze memory growth trend
                if len(memory_samples) >= 3:
                    growth_rate = (memory_samples[-1] - memory_samples[0]) / len(memory_samples)
                    print(f"  Memory growth rate: {growth_rate:.3f} MB per 100 messages")
                    
                    # Assert reasonable memory growth
                    assert growth_rate < 1.0, f"Memory growth rate too high: {growth_rate:.3f} MB per 100 messages"
                
            finally:
                await fix_engine.stop()
    
    @pytest.mark.asyncio
    async def test_long_running_session_memory_stability(self, fix_engine, mock_application):
        """Test memory stability during long-running sessions."""
        
        fix_engine.application = mock_application
        
        with memory_monitor() as monitor:
            try:
                await fix_engine.start()
                await asyncio.sleep(0.5)
                
                # Run for simulated "long" time with periodic activity
                duration = 30  # 30 seconds
                sample_interval = 5  # Sample every 5 seconds
                memory_samples = []
                
                start_time = time.time()
                
                while time.time() - start_time < duration:
                    # Send periodic messages
                    for i in range(10):
                        message = {
                            '8': 'FIX.4.4',
                            '35': '0',
                            '49': 'SENDER',
                            '56': 'TARGET',
                            '34': str(int(time.time() - start_time) * 10 + i),
                            '52': '20250726-12:00:00.000',
                        }
                        
                        try:
                            await fix_engine.send_to_target(message)
                        except Exception:
                            pass
                        
                        await asyncio.sleep(0.1)
                    
                    # Sample memory
                    current_memory = monitor['process'].memory_info().rss / 1024 / 1024
                    memory_samples.append({
                        'time': time.time() - start_time,
                        'memory': current_memory
                    })
                    
                    print(f"  Time {time.time() - start_time:.1f}s: {current_memory:.2f} MB")
                    
                    # Wait for next sample
                    await asyncio.sleep(sample_interval)
                
                # Analyze memory stability
                if len(memory_samples) >= 3:
                    memories = [sample['memory'] for sample in memory_samples]
                    
                    # Check for memory leaks (sustained growth)
                    initial_memory = memories[0]
                    final_memory = memories[-1]
                    growth = final_memory - initial_memory
                    
                    print(f"  Total memory growth: {growth:.2f} MB over {duration}s")
                    
                    # Memory should not grow significantly over time
                    max_acceptable_growth = 50  # 50MB over test duration
                    assert growth < max_acceptable_growth, f"Memory leak detected: {growth:.2f} MB growth"
                    
                    # Check for memory stability (low variance)
                    memory_variance = statistics.variance(memories)
                    print(f"  Memory variance: {memory_variance:.2f}")
                    
                    # Variance should be reasonable
                    assert memory_variance < 100, f"Memory too unstable: variance {memory_variance:.2f}"
            
            finally:
                await fix_engine.stop()


@pytest.mark.performance
@pytest.mark.slow  
class TestBenchmarking:
    """Benchmarking tests for performance measurement."""
    
    @pytest.mark.asyncio
    async def test_message_throughput_benchmark(self, fix_engine_pair, mock_application):
        """Benchmark message throughput under optimal conditions."""
        initiator, acceptor = fix_engine_pair
        
        # Track throughput metrics
        throughput_results = {
            'sent_count': 0,
            'received_count': 0,
            'start_time': None,
            'end_time': None,
            'latencies': []
        }
        
        message_timestamps = {}
        
        async def track_received_messages(session_id, message):
            msg_id = message.get('11', message.get('34'))
            if msg_id in message_timestamps:
                receive_time = time.time()
                send_time = message_timestamps[msg_id]
                latency = (receive_time - send_time) * 1000  # ms
                throughput_results['latencies'].append(latency)
            
            throughput_results['received_count'] += 1
        
        mock_application.from_app = track_received_messages
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start sessions
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        await asyncio.sleep(1.0)
        
        try:
            if initiator.is_logged_on():
                # Benchmark parameters
                message_count = 5000
                batch_size = 50
                
                print(f"\n{'='*60}")
                print(f"THROUGHPUT BENCHMARK")
                print(f"{'='*60}")
                print(f"Target messages: {message_count}")
                print(f"Batch size: {batch_size}")
                
                throughput_results['start_time'] = time.time()
                
                # Send messages in batches for better performance
                for batch_start in range(0, message_count, batch_size):
                    batch_tasks = []
                    
                    for i in range(batch_size):
                        if batch_start + i >= message_count:
                            break
                        
                        msg_id = f'MSG{batch_start + i}'
                        message = {
                            '8': 'FIX.4.4',
                            '35': 'D',  # NewOrderSingle
                            '49': 'INITIATOR',
                            '56': 'ACCEPTOR',
                            '34': str(batch_start + i + 2),
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
                        
                        # Create send task
                        batch_tasks.append(initiator.send_to_target(message))
                    
                    # Send batch
                    await asyncio.gather(*batch_tasks, return_exceptions=True)
                    throughput_results['sent_count'] += len(batch_tasks)
                    
                    # Progress update
                    if batch_start % (batch_size * 10) == 0:
                        progress = (batch_start / message_count) * 100
                        print(f"  Progress: {progress:.1f}% ({batch_start}/{message_count})")
                        await asyncio.sleep(0.01)  # Brief pause
                
                throughput_results['end_time'] = time.time()
                
                # Wait for message processing
                await asyncio.sleep(3.0)
                
                # Calculate and display results
                test_duration = throughput_results['end_time'] - throughput_results['start_time']
                send_throughput = throughput_results['sent_count'] / test_duration
                receive_throughput = throughput_results['received_count'] / test_duration
                
                print(f"\nBenchmark Results:")
                print(f"  Test duration: {test_duration:.2f} seconds")
                print(f"  Messages sent: {throughput_results['sent_count']}")
                print(f"  Messages received: {throughput_results['received_count']}")
                print(f"  Send throughput: {send_throughput:.2f} msg/sec")
                print(f"  Receive throughput: {receive_throughput:.2f} msg/sec")
                print(f"  Message loss rate: {((throughput_results['sent_count'] - throughput_results['received_count']) / throughput_results['sent_count'] * 100):.2f}%")
                
                if throughput_results['latencies']:
                    avg_latency = statistics.mean(throughput_results['latencies'])
                    p95_latency = sorted(throughput_results['latencies'])[int(len(throughput_results['latencies']) * 0.95)]
                    p99_latency = sorted(throughput_results['latencies'])[int(len(throughput_results['latencies']) * 0.99)]
                    
                    print(f"  Average latency: {avg_latency:.2f} ms")
                    print(f"  95th percentile latency: {p95_latency:.2f} ms")
                    print(f"  99th percentile latency: {p99_latency:.2f} ms")
                
                # Performance assertions
                assert send_throughput >= 500, f"Send throughput too low: {send_throughput:.2f} msg/sec"
                
                if throughput_results['latencies']:
                    assert avg_latency < 50, f"Average latency too high: {avg_latency:.2f} ms"
        
        finally:
            await initiator.stop()
            await acceptor.stop()
    
    @pytest.mark.asyncio
    async def test_cpu_usage_benchmark(self, fix_engine_pair, mock_application):
        """Benchmark CPU usage under load."""
        initiator, acceptor = fix_engine_pair
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start sessions
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        await asyncio.sleep(1.0)
        
        try:
            if initiator.is_logged_on():
                print(f"\n{'='*60}")
                print(f"CPU USAGE BENCHMARK")
                print(f"{'='*60}")
                
                # Monitor CPU usage
                process = psutil.Process()
                cpu_samples = []
                
                async def monitor_cpu():
                    for _ in range(20):  # 20 seconds
                        cpu_percent = process.cpu_percent(interval=1.0)
                        cpu_samples.append(cpu_percent)
                        print(f"  CPU usage: {cpu_percent:.1f}%")
                
                # Start CPU monitoring
                monitor_task = asyncio.create_task(monitor_cpu())
                
                # Generate load
                message_count = 3000
                
                for i in range(message_count):
                    message = {
                        '8': 'FIX.4.4',
                        '35': 'D',
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
                    
                    # Control rate
                    if i % 200 == 0:
                        await asyncio.sleep(0.01)
                
                # Wait for monitoring to complete
                await monitor_task
                
                # Analyze CPU usage
                if cpu_samples:
                    avg_cpu = statistics.mean(cpu_samples)
                    max_cpu = max(cpu_samples)
                    
                    print(f"\nCPU Benchmark Results:")
                    print(f"  Average CPU usage: {avg_cpu:.2f}%")
                    print(f"  Peak CPU usage: {max_cpu:.2f}%")
                    print(f"  Messages processed: {message_count}")
                    
                    # CPU usage assertions
                    assert avg_cpu < 70, f"Average CPU usage too high: {avg_cpu:.2f}%"
                    assert max_cpu < 90, f"Peak CPU usage too high: {max_cpu:.2f}%"
        
        finally:
            await initiator.stop()
            await acceptor.stop()
    
    @pytest.mark.asyncio
    async def test_concurrent_sessions_benchmark(self, sample_config_dict, free_port):
        """Benchmark performance with multiple concurrent sessions."""
        session_count = 3  # Limited for test environment
        messages_per_session = 500
        
        print(f"\n{'='*60}")
        print(f"CONCURRENT SESSIONS BENCHMARK")
        print(f"{'='*60}")
        print(f"Sessions: {session_count}")
        print(f"Messages per session: {messages_per_session}")
        
        # Performance tracking
        session_metrics = {}
        
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
        acceptor.application = Mock(spec=Application)
        
        await acceptor.start()
        await asyncio.sleep(0.1)
        
        sessions = []
        
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
                sessions.append((i, initiator, config_path))
                
                await initiator.start()
                await asyncio.sleep(0.1)
            
            # Wait for all sessions to establish
            await asyncio.sleep(2.0)
            
            # Verify sessions
            connected_sessions = 0
            for i, initiator, _ in sessions:
                if initiator.is_logged_on():
                    connected_sessions += 1
            
            print(f"  Connected sessions: {connected_sessions}/{session_count}")
            
            # Start benchmark
            start_time = time.time()
            
            async def benchmark_session(session_id, initiator):
                """Benchmark individual session."""
                session_start = time.time()
                sent_count = 0
                
                for i in range(messages_per_session):
                    message = {
                        '8': 'FIX.4.4',
                        '35': '0',  # Heartbeat
                        '49': f'INIT{session_id}',
                        '56': 'ACCEPTOR',
                        '34': str(i + 2),
                        '52': '20250726-12:00:00.000',
                    }
                    
                    try:
                        await initiator.send_to_target(message)
                        sent_count += 1
                    except Exception as e:
                        print(f"  Session {session_id} send failed: {e}")
                    
                    # Control rate
                    if i % 50 == 0:
                        await asyncio.sleep(0.01)
                
                session_end = time.time()
                session_duration = session_end - session_start
                session_throughput = sent_count / session_duration
                
                session_metrics[session_id] = {
                    'duration': session_duration,
                    'sent_count': sent_count,
                    'throughput': session_throughput
                }
                
                print(f"  Session {session_id}: {session_throughput:.2f} msg/sec")
            
            # Run all sessions concurrently
            benchmark_tasks = []
            for i, initiator, _ in sessions:
                benchmark_tasks.append(benchmark_session(i, initiator))
            
            await asyncio.gather(*benchmark_tasks, return_exceptions=True)
            
            end_time = time.time()
            total_duration = end_time - start_time
            
            # Calculate aggregate metrics
            total_sent = sum(metrics['sent_count'] for metrics in session_metrics.values())
            aggregate_throughput = total_sent / total_duration
            
            session_throughputs = [metrics['throughput'] for metrics in session_metrics.values()]
            avg_session_throughput = statistics.mean(session_throughputs) if session_throughputs else 0
            
            print(f"\nConcurrent Sessions Benchmark Results:")
            print(f"  Total duration: {total_duration:.2f} seconds")
            print(f"  Total messages sent: {total_sent}")
            print(f"  Aggregate throughput: {aggregate_throughput:.2f} msg/sec")
            print(f"  Average per-session throughput: {avg_session_throughput:.2f} msg/sec")
            
            # Performance assertions
            assert aggregate_throughput >= 200, f"Aggregate throughput too low: {aggregate_throughput:.2f} msg/sec"
            
        finally:
            # Cleanup
            for i, initiator, config_path in sessions:
                if initiator._running:
                    await initiator.stop()
                os.unlink(config_path)
            
            await acceptor.stop()
            os.unlink(acceptor_config_path)


if __name__ == '__main__':
    pytest.main([__file__, '-v', '--tb=short', '-m', 'performance'])
