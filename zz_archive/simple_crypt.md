# SimpleCrypt Usage Guide

## Overview

`SimpleCrypt` is a utility class for **obfuscating passwords or secrets in configuration files** using only Python standard library modules.  
It uses PBKDF2 for key derivation, a hash-based stream cipher for encryption, and HMAC for integrity.  
**Note:** This is for obfuscation only, not strong cryptographic security.

---

## How to Use

### 1. Encrypt a Password for Your Config File

```python
from pyfixmsg_plus.fixcypt.simple_crypt import SimpleCrypt

# Set your encryption password (master password)
crypt = SimpleCrypt("my_master_password")

# Encrypt a password (for example, before putting it in config)
encrypted = crypt.check_crypt("clear:my_db_password")
print("Encrypted password for config:", encrypted)
```

Copy the output and use it in your config file instead of the plain password.

---

### 2. Decrypt a Password from Your Config File

Suppose your config file contains:
```
db_password = <encrypted_value>
```

In your application:

```python
from pyfixmsg_plus.fixcypt.simple_crypt import SimpleCrypt

# Use the same master password as above
crypt = SimpleCrypt("my_master_password")

# Read the encrypted value from config
encrypted = "<encrypted_value_from_config>"

# Decrypt it for use
decrypted_password = crypt.check_crypt(encrypted)
print("Decrypted password:", decrypted_password)
```

---

### 3. Typical Integration in a FIX Application

```python
from pyfixmsg_plus.fixcypt.simple_crypt import SimpleCrypt

# Initialize crypt with your master password (from env, prompt, etc.)
crypt = SimpleCrypt("my_master_password")

# When loading config:
db_password = config.get("db_password")
real_password = crypt.check_crypt(db_password)
# Use real_password to connect to your DB or service
```

---

## Notes

- The encryption is for **obfuscation only**, not strong security.
- Always keep your master password safe and do not hardcode it in production code.
- The salt is automatically generated and stored with the encrypted value.
- The same master password must be used for both encryption and decryption.

---

## API Reference

### `SimpleCrypt(crypt_pass, event_notifier=None, logger=None, iterations=100_000)`

- `crypt_pass`: The master password for encryption/decryption.
- `event_notifier`: Optional notifier for logging events.
- `logger`: Optional logger.
- `iterations`: PBKDF2 iterations (default: 100,000).

### `check_crypt(param: str) -> str`

- If `param` starts with `"clear:"`, encrypts the value after the colon.
- Otherwise, decrypts the value.

---

## Security Warning

This method is **not suitable for protecting highly sensitive data**.  
It is intended for hiding passwords in config files from casual inspection, not for strong cryptographic protection.
