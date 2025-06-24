## dto.TLS
TLS represents the Aerospike cluster TLS configuration options.

| Field               | Description                                                                            | Default Value |
|---------------------|----------------------------------------------------------------------------------------|---------------|
| `ca-file`           | Path to a trusted CA certificate file.                                                 |               |
| `ca-path`           | Path to a directory of trusted CA certificates.                                        |               |
| `cert-file`         | Path to the chain file for mutual authentication (if Aerospike Cluster supports it).   |               |
| `cipher-suite`      | TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format. |               |
| `key-file`          | Path to the key for mutual authentication (if Aerospike cluster supports it).          |               |
| `key-file-password` | Password to load protected TLS-keyfile (env:VAR, file:PATH, PASSWORD).                 |               |
| `name`              | The default TLS name used to authenticate each TLS socket connection.                  |               |
| `protocols`         | TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.     | `TLSv1.2`     |
