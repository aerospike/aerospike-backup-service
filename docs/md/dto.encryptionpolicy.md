## dto.EncryptionPolicy

| Field        | Description                                                                 | Default Value | Possible Values            |
|--------------|-----------------------------------------------------------------------------|---------------|----------------------------|
| `key-env`    | The name of the environment variable containing the encryption key.         |               |                            |
| `key-file`   | The path to the file containing the encryption key.                         |               |                            |
| `key-secret` | The secret keyword in Aerospike Secret Agent containing the encryption key. |               |                            |
| `mode`       | The encryption mode to be used (NONE, AES128, AES256)                       | `NONE`        | `NONE`, `AES128`, `AES256` |
