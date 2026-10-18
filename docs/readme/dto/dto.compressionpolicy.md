## dto.CompressionPolicy
CompressionPolicy contains backup compression information.

| Field       | Description                                                                                                                                                                                                                                                                  | Possible Values |
|-------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------|
| 📍 `mode`    | The compression mode to be used. Required.                                                                                                                                                                                                                                   | `NONE`, `ZSTD`  |
| `level`     | The compression level to use, from -1 to 22.<br>A higher value gives better compression at the cost of speed,<br>but not every step changes the result: neighboring levels may compress the same way.<br>Required for ZSTD; must not be set if the compression mode is NONE. |                 |

📍 = Required field