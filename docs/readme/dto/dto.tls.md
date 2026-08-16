## dto.TLS
TLS represents the Aerospike cluster TLS configuration options.

| Field               | Description                                                                                      | Default Value |
|---------------------|--------------------------------------------------------------------------------------------------|---------------|
| `ca-file`           | Path to a trusted CA certificate file in PEM format.                                             |               |
| `ca-path`           | Path to a directory of trusted CA certificates.                                                  |               |
| `cert-file`         | Path to a client certificate file for mutual TLS authentication.                                 |               |
| `cipher-suite`      | TLS cipher selection criteria. The format is the same as OpenSSL's Cipher List Format.           |               |
| `key-file`          | Path to a client private key file for mutual TLS authentication.                                 |               |
| `key-file-password` | Passphrase for an encrypted TLS key file. The value is used verbatim as the decryption password. |               |
| `name`              | TLSName used for server certificate verification (ServerName for SNI).                           |               |
| `protocols`         | TLS protocol selection criteria. This format is the same as Apache's SSL Protocol.               | `TLSv1.2`     |
