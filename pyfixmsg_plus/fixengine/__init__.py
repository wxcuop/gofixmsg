import warnings

warnings.warn(
    "The Python FIX engine is deprecated and will be removed in a future release. "
    "Please migrate to the Go-based implementation in the 'gorewrite' directory.",
    DeprecationWarning,
    stacklevel=2
)

from .engine import FixEngine
from .configmanager import ConfigManager
from .heartbeat import Heartbeat
from .heartbeat_builder import HeartbeatBuilder
from .testrequest import TestRequest
from .network import Acceptor, Initiator
from .message_handler import (
    MessageProcessor,
    MessageHandler,
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
    HeartbeatHandler,
)
from .message_store_factory import MessageStoreFactory
from .state_machine import StateMachine, Disconnected
from .scheduler import Scheduler
from .simple_crypt import SimpleCrypt

__all__ = [
    "FixEngine",
    "ConfigManager",
    "Heartbeat",
    "HeartbeatBuilder",
    "TestRequest",
    "Acceptor",
    "Initiator",
    "MessageProcessor",
    "MessageHandler",
    "LogonHandler",
    "TestRequestHandler",
    "ExecutionReportHandler",
    "NewOrderHandler",
    "CancelOrderHandler",
    "OrderCancelReplaceHandler",
    "OrderCancelRejectHandler",
    "NewOrderMultilegHandler",
    "MultilegOrderCancelReplaceHandler",
    "ResendRequestHandler",
    "SequenceResetHandler",
    "RejectHandler",
    "LogoutHandler",
    "HeartbeatHandler",
    "MessageStoreFactory",
    "StateMachine",
    "Disconnected",
    "Scheduler",
    "SimpleCrypt",
]
