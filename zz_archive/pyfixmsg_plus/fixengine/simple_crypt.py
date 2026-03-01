import os
import hashlib
import base64
from hmac import compare_digest
from typing import Optional, Any

class SimpleCryptException(Exception):
    pass

class SimpleCryptEvents:
    def notify(self, msg: str, level: str) -> None:
        raise NotImplementedError('Subclass responsibility')

class SimpleCryptEventsNotifier:
    def __init__(self, event_handler: SimpleCryptEvents) -> None:
        self.event_handler = event_handler

    def notify(self, msg: str, level: str) -> None:
        self.event_handler.notify(msg, level)

class SimpleCrypt:
    """
    Symmetric encryption using only Python standard library (PBKDF2 + hash-based stream cipher + HMAC).
    """
    def __init__(
        self,
        crypt_pass: str,
        event_notifier: Optional[SimpleCryptEventsNotifier] = None,
        logger: Optional[Any] = None,
        iterations: int = 100_000
    ) -> None:
        self.crypt_pass = crypt_pass
        self.event_notifier = event_notifier
        self.logger = logger
        self.iterations = iterations

    def log_message(self, msg: str, level: str) -> None:
        if self.event_notifier:
            self.event_notifier.notify(msg, level)
        if self.logger:
            log_methods = {
                "DEBUG": self.logger.debug,
                "ERROR": self.logger.error,
                "FATAL": self.logger.fatal,
                "INFO": self.logger.info,
                "WARNING": getattr(self.logger, "warning", self.logger.info)
            }
            log_methods.get(level, self.logger.info)(msg)

    def check_crypt(self, param: str) -> str:
        crypt_password = self.crypt_pass.encode('utf-8')
        if param.startswith("clear:"):
            self.log_message(f"{param}, will be encrypted", "INFO")
            encrypted_value = self.encrypt(crypt_password, param.split(':', 1)[1])
            self.log_message(f"Encrypted value is:\n{encrypted_value}", "INFO")
            return encrypted_value
        else:
            self.log_message(f"Decrypting {param}", "INFO")
            return self.decrypt(crypt_password, param)

    def derive_key(self, password: bytes, salt: bytes) -> bytes:
        return hashlib.pbkdf2_hmac('sha256', password, salt, self.iterations, dklen=32)

    def stream_cipher(self, key: bytes, data: bytes) -> bytes:
        # Hash-based keystream (not secure, but better than plain XOR)
        output = bytearray()
        counter = 0
        for i in range(0, len(data), 32):
            block = data[i:i+32]
            keystream = hashlib.sha256(key + counter.to_bytes(4, 'big')).digest()
            output.extend([b ^ k for b, k in zip(block, keystream)])
            counter += 1
        return bytes(output)

    def encrypt(self, password: bytes, plaintext: str) -> str:
        salt = os.urandom(16)
        key = self.derive_key(password, salt)
        plaintext_bytes = plaintext.encode('utf-8')
        ciphertext = self.stream_cipher(key, plaintext_bytes)
        mac = hashlib.pbkdf2_hmac('sha256', key, ciphertext, 1, dklen=32)
        return base64.b64encode(salt + mac + ciphertext).decode('utf-8')

    def decrypt(self, password: bytes, text: str) -> str:
        data = base64.b64decode(text)
        salt, mac, ciphertext = data[:16], data[16:48], data[48:]
        key = self.derive_key(password, salt)
        expected_mac = hashlib.pbkdf2_hmac('sha256', key, ciphertext, 1, dklen=32)
        if not compare_digest(mac, expected_mac):
            raise SimpleCryptException("Decryption failed: MAC does not match (data tampered or wrong password)")
        plaintext_bytes = self.stream_cipher(key, ciphertext)
        try:
            return plaintext_bytes.decode('utf-8')
        except Exception as e:
            raise SimpleCryptException(f"Decryption failed: {e}")

if __name__ == "__main__":
    # Example usage
    crypt = SimpleCrypt("my_master_password")
    encrypted = crypt.check_crypt("clear:my_db_password")
    print("Encrypted password for config:", encrypted)

    decrypted = crypt.check_crypt(encrypted)
    print("Decrypted password:", decrypted)
    
    # This should match the original clear text
    assert decrypted == "my_db_password"