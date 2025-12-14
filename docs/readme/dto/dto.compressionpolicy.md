## dto.CompressionPolicy
CompressionPolicy contains backup compression information.

| Field   | Description                                                                                                                                                          | Default Value | Possible Values |
|---------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|-----------------|
| `level` | The compression level to use.<br>Algorithm-specific; for zstd: from -1 (fastest) to 22 (best compression).<br>This field is ignored if the compression mode is NONE. | `0`           |                 |
| `mode`  | The compression mode to be used (default is NONE).                                                                                                                   | `NONE`        | `NONE`, `ZSTD`  |
