from pyfixmsg_plus.fixengine.heartbeat import Heartbeat

class HeartbeatBuilder:
    def __init__(self):
        self.send_message_callback = None
        self.config_manager = None
        self.heartbeat_interval = 30
        self.state_machine = None
        self.fix_engine = None

    def set_send_message_callback(self, send_message_callback):
        self.send_message_callback = send_message_callback
        return self

    def set_config_manager(self, config_manager):
        self.config_manager = config_manager
        return self

    def set_heartbeat_interval(self, heartbeat_interval):
        self.heartbeat_interval = heartbeat_interval
        return self

    def set_state_machine(self, state_machine):
        self.state_machine = state_machine
        return self

    def set_fix_engine(self, fix_engine):
        self.fix_engine = fix_engine
        return self

    def build(self):
        return Heartbeat(self.send_message_callback, self.config_manager, self.heartbeat_interval, self.state_machine, self.fix_engine)
