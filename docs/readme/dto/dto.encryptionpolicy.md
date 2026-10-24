## dto.EncryptionPolicy
EncryptionPolicy contains backup encryption information.

| Field             | Description                                                                                                                                                                                                                                                                | Possible Values            |
|-------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------|
| 📍 `mode`          | The encryption mode to be used (NONE, AES128, AES256). Required.                                                                                                                                                                                                           | `NONE`, `AES128`, `AES256` |
| `key-env`         | The name of the environment variable containing the encryption key.                                                                                                                                                                                                        |                            |
| `key-file`        | The path to the file containing the encryption key.                                                                                                                                                                                                                        |                            |
| 🔒 `key-secret`    | The secret keyword in Aerospike Secret Agent containing the encryption key.<br>This is sensitive information. Can be a path in secret agent or an actual value.<br>Literal values are redacted as "[secret]" in API responses; secret agent references are returned as-is. |                            |

📍 = Required field
🔒 = Redacted in API responses