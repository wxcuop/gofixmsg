from abc import ABC, abstractmethod
from typing import Any

class Application(ABC):
    @abstractmethod
    def onCreate(self, sessionID: str) -> None:
        """Called when a session is created."""
        pass

    @abstractmethod
    def onLogon(self, sessionID: str) -> None:
        """Called when a session logs on."""
        pass

    @abstractmethod
    def onLogout(self, sessionID: str) -> None:
        """Called when a session logs out."""
        pass

    @abstractmethod
    def toAdmin(self, message: Any, sessionID: str) -> None:
        """Called before an admin message is sent."""
        pass

    @abstractmethod
    def fromAdmin(self, message: Any, sessionID: str) -> None:
        """Called when an admin message is received."""
        pass

    @abstractmethod
    def toApp(self, message: Any, sessionID: str) -> None:
        """Called before an application message is sent."""
        pass

    @abstractmethod
    def fromApp(self, message: Any, sessionID: str) -> None:
        """Called when an application message is received."""
        pass

    @abstractmethod
    def onMessage(self, message: Any, sessionID: str) -> None:
        """Called for every application-level message."""
        pass
