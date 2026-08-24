## dto.SecretAgent
SecretAgent represents the configuration of an Aerospike Secret Agent.

| Field                  | Description                                                          | Default Value | Possible Values |
|------------------------|----------------------------------------------------------------------|---------------|-----------------|
| 📍 `address`            | Address of the Secret Agent.                                         |               |                 |
| 📍 `connection-type`    | Connection type.                                                     |               | `tcp`, `unix`   |
| `ca-file`              | Path to a trusted CA certificate file in PEM format.                 |               |                 |
| `cert-file`            | Path to a client certificate file for mutual TLS authentication.     |               |                 |
| `is-base64`            | Flag that shows if secret agent responses are encrypted with base64. | `false`       |                 |
| `key-file`             | Path to a client private key file for mutual TLS authentication.     |               |                 |
| `name`                 | TLS ServerName (SNI) for verifying the peer certificate.             |               |                 |
| `port`                 | Port the Secret Agent is running on.                                 |               |                 |
| `timeout`              | Timeout in milliseconds.                                             | `1000`        |                 |

📍 = Required field