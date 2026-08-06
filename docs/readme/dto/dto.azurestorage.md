## dto.AzureStorage
AzureStorage represents the configuration for Azure Blob storage.

| Field                 | Description                                                                                                                                                                          | Default Value |
|-----------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| 📍 `container-name`    | ContainerName is the name of the Azure Blob container.                                                                                                                               |               |
| 📍 `endpoint`          | Endpoint is the Azure Blob service endpoint URL.                                                                                                                                     |               |
| `account-key`         | AccountKey is the Azure storage account key for Shared Key authentication.<br>This is sensitive information. Can be a path in secret agent or an actual value.                       |               |
| `account-name`        | AccountName is the Azure storage account name for Shared Key authentication.                                                                                                         |               |
| `client-id`           | ClientID is the Azure Active Directory client ID for AAD authentication.                                                                                                             |               |
| `client-secret`       | ClientSecret is the Azure Active Directory client secret for AAD authentication.<br>This is sensitive information. Can be a path in secret agent or an actual value.                 |               |
| `min-part-size`       | The minimum size in bytes of individual Azure Blob chunks.                                                                                                                           | `5.24288e+07` |
| `path`                | Path is the root path for the backup repository within the container.<br>This path is relative to the container.<br>If not specified, backups will be saved in the container's root. |               |
| `secret-agent`        | Secret Agent configuration (optional).<br>Mutually exclusive with 'secret-agent-name'.<br>See: [dto.SecretAgent](dto.secretagent.md)                                                 |               |
| `secret-agent-name`   | Secret Agent configuration (optional). Link to one of preconfigured agents.<br>Mutually exclusive with 'secret-agent'.                                                               |               |
| `storage-class`       | StorageClass defines the storage tier for data and metadata objects.<br>See: [dto.AzureStorageClass](dto.azurestorageclass.md)                                                       |               |
| `tenant-id`           | TenantID is the Azure Active Directory tenant ID for AAD authentication.                                                                                                             |               |

📍 = Required field