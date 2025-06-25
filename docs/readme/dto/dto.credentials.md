## dto.Credentials
Credentials represents authentication details to the Aerospike cluster.

| Field               | Description                                                                                                                          | Default Value | Possible Values               |
|---------------------|--------------------------------------------------------------------------------------------------------------------------------------|---------------|-------------------------------|
| `auth-mode`         | The authentication mode string (INTERNAL, EXTERNAL, PKI).                                                                            | `INTERNAL`    | `INTERNAL`, `EXTERNAL`, `PKI` |
| `password`          | The password for the cluster authentication.<br>It can be either plain text or path into the secret agent.                           |               |                               |
| `password-path`     | The file path with the password string.                                                                                              |               |                               |
| `secret-agent`      | Secret Agent configuration (optional).<br>Mutually exclusive with 'secret-agent-name'.<br>See: [dto.SecretAgent](dto.secretagent.md) |               |                               |
| `secret-agent-name` | Secret Agent configuration (optional). Link to one of preconfigured agents.<br>Mutually exclusive with 'secret-agent'.               |               |                               |
| `user`              | The username for the cluster authentication.                                                                                         |               |                               |
