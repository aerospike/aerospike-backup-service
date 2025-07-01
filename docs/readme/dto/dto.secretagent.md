## dto.SecretAgent
SecretAgent represents the configuration of an Aerospike Secret Agent.

| Field                  | Description                                                          | Default Value | Possible Values |
|------------------------|----------------------------------------------------------------------|---------------|-----------------|
| 📍 `address`            | Address of the Secret Agent.                                         |               |                 |
| 📍 `connection-type`    | Connection type.                                                     |               | `tcp`, `unix`   |
| `is-base64`            | Flag that shows if secret agent responses are encrypted with base64. | `false`       |                 |
| `port`                 | Port the Secret Agent is running on.                                 |               |                 |
| `timeout`              | Timeout in milliseconds.                                             | `1000`        |                 |
| `tls-ca-file`          | The path to a trusted CA certificate file in PEM format.             |               |                 |

📍 = Required field