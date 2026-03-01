"""
Enhanced Test Cleanup Utilities for PyFixMsg Plus
Provides utilities to handle async task cleanup and prevent pending task warnings.
"""
import asyncio
import warnings
from contextlib import asynccontextmanager
from typing import AsyncGenerator
from unittest.mock import Mock, AsyncMock, patch

class TestCleanupManager:
    """Manages async task cleanup for tests to prevent pending task warnings."""
    
    def __init__(self):
        self.created_tasks = []
        self.original_create_task = None
        
    def track_task(self, task):
        """Track a task for cleanup."""
        self.created_tasks.append(task)
        return task
        
    async def cleanup_tasks(self):
        """Clean up all tracked tasks."""
        for task in self.created_tasks:
            if not task.done():
                task.cancel()
                try:
                    await task
                except asyncio.CancelledError:
                    pass
                except Exception:
                    pass  # Ignore other exceptions during cleanup
        self.created_tasks.clear()
        
    @asynccontextmanager
    async def managed_engine(self, engine):
        """Context manager for properly cleaning up engine resources."""
        try:
            yield engine
        finally:
            # Clean up scheduler task if it exists
            if hasattr(engine, 'scheduler') and hasattr(engine.scheduler, 'scheduler_task'):
                if engine.scheduler.scheduler_task and not engine.scheduler.scheduler_task.done():
                    engine.scheduler.scheduler_task.cancel()
                    try:
                        await engine.scheduler.scheduler_task
                    except asyncio.CancelledError:
                        pass
            
            # Clean up any other engine tasks
            if hasattr(engine, 'scheduler_task') and engine.scheduler_task:
                if not engine.scheduler_task.done():
                    engine.scheduler_task.cancel()
                    try:
                        await engine.scheduler_task
                    except asyncio.CancelledError:
                        pass


# Global cleanup manager instance
cleanup_manager = TestCleanupManager()


def suppress_task_warnings():
    """Suppress specific asyncio task warnings during tests."""
    warnings.filterwarnings("ignore", message="Task was destroyed but it is pending!")
    warnings.filterwarnings("ignore", message=".*Task.*destroyed.*pending.*", category=RuntimeWarning)


def mock_scheduler_without_tasks():
    """Create a mock scheduler that doesn't create background tasks."""
    mock_scheduler = Mock()
    mock_scheduler.scheduler_task = None
    mock_scheduler.start = AsyncMock()
    mock_scheduler.stop = AsyncMock()
    mock_scheduler.reset = AsyncMock()
    mock_scheduler.reset_start = AsyncMock()
    mock_scheduler.run_scheduler = AsyncMock()
    mock_scheduler.load_configuration = Mock()
    return mock_scheduler


@asynccontextmanager
async def clean_engine_context(engine_factory, *args, **kwargs):
    """
    Context manager that creates an engine and ensures proper cleanup.
    
    Usage:
        async with clean_engine_context(FixEngine, config_manager, app) as engine:
            # Use engine
            pass
        # Engine is automatically cleaned up
    """
    engine = None
    try:
        engine = engine_factory(*args, **kwargs)
        async with cleanup_manager.managed_engine(engine) as managed_engine:
            yield managed_engine
    finally:
        if engine:
            # Additional cleanup if needed
            pass
