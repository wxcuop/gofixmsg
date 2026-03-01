import configparser
from pyfixmsg_plus.fixengine.simple_crypt import SimpleCrypt

# Provide module-level encrypt/decrypt helpers using a default salt
_DEFAULT_CRYPT_SALT = "seasalt_is_salty"
_simple_crypt = SimpleCrypt(_DEFAULT_CRYPT_SALT)

def encrypt(value):
    return _simple_crypt.encrypt(_DEFAULT_CRYPT_SALT.encode('utf-8'), value)

def decrypt(value):
    return _simple_crypt.decrypt(_DEFAULT_CRYPT_SALT.encode('utf-8'), value)

# Ensure there's only one instance of ConfigManager (Singleton).
class ConfigManager:
    _instance = None

    def __new__(cls, *args, **kwargs):
        if not cls._instance:
            cls._instance = super().__new__(cls)
        return cls._instance

    def __init__(self, config_path='config.ini'):
        if not hasattr(self, 'initialized'):
            self.config = configparser.ConfigParser()
            self.config_path = config_path
            self.load_config()
            self.initialized = True

    def load_config(self):
        try:
            self.config.read(self.config_path)
        except FileNotFoundError:
            print(f"Warning: Configuration file '{self.config_path}' not found. Using default settings.")

    def save_config(self):
        try:
            with open(self.config_path, 'w') as configfile:
                self.config.write(configfile)
        except Exception as e:
            print(f"Error saving configuration: {e}")

    def get(self, section, option, fallback=None, decrypt_value=False):
        try:
            value = self.config.get(section, option, fallback=fallback)
            if decrypt_value and value and value.startswith("ENC:"):
                return decrypt(value[4:])
            return value
        except (configparser.NoSectionError, configparser.NoOptionError):
            print(f"Warning: Section '{section}' or option '{option}' not found. Returning fallback value.")
            return fallback

    def get_message_store_type(self, fallback='database'):
        """Gets the message store type from the config, e.g., 'database' or 'aiosqlite'."""
        return self.get('FIX', 'message_store_type', fallback=fallback)

    def set(self, section, option, value, encrypt_value=False):
        if encrypt_value:
            value = "ENC:" + encrypt(value)
        if not self.config.has_section(section):
            self.config.add_section(section)
        self.config.set(section, option, value)

    def delete(self, section, option=None):
        if option:
            if self.config.has_section(section) and self.config.has_option(section, option):
                self.config.remove_option(section, option)
            else:
                print(f"Warning: Section '{section}' or option '{option}' not found.")
        else:
            if self.config.has_section(section):
                self.config.remove_section(section)
            else:
                print(f"Warning: Section '{section}' not found.")

    def reset(self):
        self.config = configparser.ConfigParser()
        self.save_config()

# Example usage
if __name__ == "__main__":
    cm1 = ConfigManager()
    cm2 = ConfigManager()
    cm1.load_config()

    # Set and get a plain value
    cm1.set('FIX', 'new_option', 'new_value')
    print("Plain value:", cm1.get('FIX', 'new_option'))

    # Set and get an encrypted value
    cm1.set('FIX', 'password', 'supersecret', encrypt_value=True)
    print("Encrypted (raw):", cm1.get('FIX', 'password'))
    print("Decrypted:", cm1.get('FIX', 'password', decrypt_value=True))

    cm1.save_config()

    # Delete the encrypted option
    cm2.delete('FIX', 'password')
    cm2.save_config()

    # Reset config
    cm1.reset()
    cm1.save_config()

    # Check if both instances are the same
    print("Singleton check:", cm1 is cm2)  # Should print: True
