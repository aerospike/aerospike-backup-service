## dto.AzureStorageClass
AzureStorageClass represents the configuration for Azure Blob Storage access tiers.

| Field      | Description                                            | Possible Values                  |
|------------|--------------------------------------------------------|----------------------------------|
| `data`     | DataClass specifies the storage tier for object data.  | `Hot`, `Cool`, `Cold`, `Archive` |
| `metadata` | MetadataClass specifies the storage tier for metadata. | `Hot`, `Cool`, `Cold`            |
