# Security

Aerospike Backup Service (ABS) supports securely retrieving sensitive credentials—such as cluster passwords, storage credentials, and encryption keys—from a configured Secret Agent.

## Credential Handling

Credentials can be provided in one of the following ways:

* **Literal values** in the configuration file. This is **not recommended for production**.
* **File-based credentials**, such as `password-path` for cluster credentials.
* **Secret Agent references**, using values prefixed with `secrets:`. These values are resolved through the configured Secret Agent.

When a Secret Agent is configured, ABS resolves secret references through its secret resolver. Credentials are kept available only for the period required by backup and restore operations, so credential changes may not take effect immediately.

## Credential Rotation

The time at which a credential change takes effect depends on the type of credential:

* **Storage credentials:** Changes take effect when the service refreshes its storage connection, which can take up to 10 minutes.
* **Cluster passwords:** Changes take effect when the existing connection is no longer in use and a new connection is established.
* **Encryption keys:** A new key is used when a new backup job starts.
* **HTTPS server certificate and key** (`service.https` `cert-file` and `key-file`): Rewrite the files at the **same paths**. The service polls both files every 10 seconds and serves a successfully loaded pair on new TLS handshakes without restarting and without dropping in-flight connections. Changing the configured paths is a static-field change and requires a restart. An invalid replacement pair (parse failure or cert/key mismatch) is ignored: the last good certificate stays in service, an error is logged, and the listener never falls back to plaintext.

## Secret Agent Availability

The Secret Agent must be available whenever ABS needs to retrieve a credential. If the Secret Agent is unavailable, new backup or restore operations that require a secret may fail.

Existing backup or restore jobs continue using the credentials that were resolved when the job started.

## Configuration

Secret Agent endpoints are configured under `secret-agents`. Clusters, storage backends, and backup routines can reference a configured Secret Agent using `secret-agent-name` or define an inline `secret-agent` block.

For configuration examples and details, see the [Configuration guide](configuration.md) 
and the [Aerospike Secret Agent documentation](https://aerospike.com/docs/tools/secret-agent).