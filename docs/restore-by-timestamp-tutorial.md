# Tutorial: How to use Restore-by-Timestamp

The Restore-by-Timestamp API allows you to restore an Aerospike database to a specific point in time. It supports both standard local restores and complex cross-region migrations without requiring a locally configured backup routine.

## Required Fields
Regardless of how you use the API, two fields are **always required**:
* `routine`: The name of the backup routine. Even if the routine isn't configured locally, this name is required because ABS uses it to construct the storage path and discover the backup chain.
* `time`: The target epoch timestamp (in milliseconds) for the recovery. ABS will apply the closest backup state prior to this timestamp.

## Optional Override Fields
You can optionally override the configuration on a field-by-field basis. If an override is omitted, ABS will attempt to fall back to the configuration defined in the local `routine`.

* **Storage Overrides:**
  * `source`: Inline definition of the storage configuration.
  * `source-name`: A reference to a pre-configured storage in the ABS config.
* **Destination Overrides:**
  * `destination`: Inline definition of the target Aerospike cluster.
  * `destination-name`: A reference to a pre-configured cluster in the ABS config.
* **Secret Agent Overrides:**
  * `secret-agent`: Inline definition of the secret agent.
  * `secret-agent-name`: A reference to a pre-configured secret agent.

*(Note: The `-name` and inline variants are mutually exclusive for each category).*

## Examples

### Example 1: Standard Restore (No Overrides)
If the routine is fully configured in your local ABS instance, you only need to provide the required fields. ABS will automatically use the storage and destination cluster defined in the "daily-backup" routine.

```json
{
  "routine": "daily-backup",
  "time": 1739538000000
}
```

### Example 2: Partial Override (Restoring to a different cluster)
In this scenario, the user wants to read from the standard storage defined in the "daily-backup" routine, but restore the data into a different cluster.

```json
{
  "routine": "daily-backup",
  "time": 1739538000000,
  "destination-name": "staging-cluster"
}
```

### Example 3: Full Override (Cross-Region Restore)
In a cross-region disaster recovery scenario, the target ABS instance might not have "daily-backup" configured locally at all. The user provides the exact S3 bucket where the copied backups live, and the local cluster to restore into. Because `source` and `destination` are fully provided, ABS skips local routine validation entirely.

```json
{
  "routine": "daily-backup",
  "time": 1739538000000,
  "source": {
    "s3": {
      "bucket": "dr-backup-bucket-us-west",
      "region": "us-west-2"
    }
  },
  "destination-name": "local-dr-cluster"
}
```

### Example 4: Full Override with Secret Agents
Secret Agents can be used in two places during a restore:
1. **Top-level:** For decrypting the backup files themselves.
2. **Destination-level:** For fetching database credentials to authenticate with the target cluster.

Both locations support referencing a globally configured secret agent by name, or defining one inline. This works perfectly even if the local backup routine doesn't exist.

```json
{
  "routine": "cross-region-routine",
  "time": 1739538000000,
  "source": {
    "s3": {
      "bucket": "dr-backup-bucket-us-west",
      "region": "us-west-2"
    }
  },
  "secret-agent-name": "my-local-decryption-agent",
  "destination": {
    "seed-nodes": [
      { "host-name": "10.0.0.5", "port": 3000 }
    ],
    "credentials": {
      "user": "admin",
      "password": "secret:my-db-password",
      "secret-agent": {
        "address": "127.0.0.1",
        "port": 3005
      }
    }
  }
}
```