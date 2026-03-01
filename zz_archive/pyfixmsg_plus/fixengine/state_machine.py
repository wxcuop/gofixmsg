import logging

class State:
    # Class attribute for the state name, useful for consistent checks
    name = "BASE_STATE" 

    def __init__(self):
        # Instance attribute for the specific name, if needed, but class attr is often better for type checks
        # self.name = self.__class__.name 
        self.logger = logging.getLogger(f"State.{self.__class__.__name__}")
        # Log instantiation of the state
        # self.logger.debug(f"Instantiated state: {self.__class__.name}")


    def on_event(self, event: str, state_machine: 'StateMachine'):
        self.logger.debug(f"Event '{event}' received in state '{self.__class__.name}'. No explicit transition defined. Staying in '{self.__class__.name}'.")
        return self # Default: no change

    def __str__(self):
        return self.__class__.name


class StateMachine:
    def __init__(self, initial_state: State):
        self.state = initial_state
        self.subscribers = []
        self.logger = logging.getLogger('StateMachine')
        self.logger.info(f"StateMachine initialized. Initial state: {self.state}")

    def on_event(self, event: str):
        old_state_name = str(self.state) # Use __str__ or state.name
        self.logger.debug(f"Event: '{event}' received by StateMachine in state: '{old_state_name}'")
        
        new_state_instance = self.state.on_event(event, self) 

        if new_state_instance is not self.state : 
            self.state = new_state_instance
            self.logger.info(f"State transition: '{old_state_name}' -> '{str(self.state)}' on event '{event}'")
            self.notify_subscribers()
        else:
            self.logger.debug(f"No state change from '{old_state_name}' on event '{event}'.")

    def subscribe(self, callback):
        self.subscribers.append(callback)

    def notify_subscribers(self):
        for callback in self.subscribers:
            try:
                callback(str(self.state)) # Pass the string name of the state
            except Exception as e:
                self.logger.error(f"Error in state change subscriber {callback}: {e}", exc_info=True)


class Disconnected(State):
    name = "DISCONNECTED"
    def __init__(self):
        super().__init__()

    def on_event(self, event: str, state_machine: StateMachine):
        if event == 'initiator_connect_attempt': 
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'Connecting'.")
            return Connecting()
        elif event == 'client_accepted_awaiting_logon': 
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'AwaitingLogon'.")
            return AwaitingLogon()
        elif event == 'initiate_reconnect': # If retry logic starts from a fully disconnected state
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'Reconnecting'.")
            return Reconnecting()
        elif event == 'force_disconnect': # Ensure it stays disconnected
            self.logger.debug(f"'{self.name}' handling '{event}': Staying 'Disconnected'.")
            return self
        return super().on_event(event, state_machine)


class Connecting(State): 
    name = "CONNECTING"
    def __init__(self):
        super().__init__()

    def on_event(self, event: str, state_machine: StateMachine):
        if event == 'connection_established': 
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'LogonInProgress'.")
            return LogonInProgress()
        elif event == 'connection_failed' or event == 'disconnect' or event == 'force_disconnect':
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'Disconnected'.")
            return Disconnected()
        return super().on_event(event, state_machine)


class LogonInProgress(State): 
    name = "LOGON_IN_PROGRESS"
    def __init__(self):
        super().__init__()

    def on_event(self, event: str, state_machine: StateMachine):
        if event == 'logon_successful': 
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'Active'.")
            return Active()
        elif event == 'logon_failed' or event == 'disconnect' or event == 'force_disconnect':
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'Disconnected'.")
            return Disconnected()
        return super().on_event(event, state_machine)


class AwaitingLogon(State): 
    name = "AWAITING_LOGON"
    def __init__(self):
        super().__init__()

    def on_event(self, event: str, state_machine: StateMachine):
        if event == 'logon_received_valid': 
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'Active'.")
            return Active()
        elif event == 'invalid_logon_received' or event == 'logon_timeout' or event == 'disconnect' or event == 'force_disconnect':
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'Disconnected'.")
            return Disconnected()
        return super().on_event(event, state_machine)


class Active(State):
    name = "ACTIVE"
    def __init__(self):
        super().__init__()

    def on_event(self, event: str, state_machine: StateMachine):
        if event == 'logout_initiated': 
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'LogoutInProgress'.")
            return LogoutInProgress()
        elif event == 'disconnect' or event == 'force_disconnect': 
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'Disconnected'.")
            return Disconnected()
        elif event == 'initiate_reconnect': 
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'Reconnecting'.")
            return Reconnecting()
        return super().on_event(event, state_machine)


class LogoutInProgress(State):
    name = "LOGOUT_IN_PROGRESS"
    def __init__(self):
        super().__init__()

    def on_event(self, event: str, state_machine: StateMachine):
        if event == 'logout_confirmed' or event == 'disconnect' or event == 'force_disconnect': 
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'Disconnected'.")
            return Disconnected()
        return super().on_event(event, state_machine)


class Reconnecting(State): 
    name = "RECONNECTING"
    def __init__(self):
        super().__init__()

    def on_event(self, event: str, state_machine: StateMachine):
        if event == 'connection_established': # Reconnect TCP attempt successful
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'LogonInProgress' (for resending Logon).")
            return LogonInProgress() 
        elif event == 'reconnect_failed_max_retries' or event == 'connection_failed' or event == 'disconnect' or event == 'force_disconnect': 
            self.logger.debug(f"'{self.name}' handling '{event}': Transitioning to 'Disconnected'.")
            return Disconnected()
        return super().on_event(event, state_machine)
