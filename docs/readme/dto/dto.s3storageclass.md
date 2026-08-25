## dto.S3StorageClass
S3StorageClass represents the configuration for S3 Storage Class.

| Field      | Description                                             | Possible Values                                                                                                                                |
|------------|---------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------|
| `data`     | DataClass specifies the storage class for object data.  | `STANDARD`, `GLACIER`, `STANDARD_IA`, `ONEZONE_IA`, `INTELLIGENT_TIERING`, `DEEP_ARCHIVE`, `OUTPOSTS`, `GLACIER_IR`, `SNOW`, `EXPRESS_ONEZONE` |
| `metadata` | MetadataClass specifies the storage class for metadata. | `STANDARD`, `STANDARD_IA`, `INTELLIGENT_TIERING`, `EXPRESS_ONEZONE`, `ONEZONE_IA`, `OUTPOSTS`                                                  |
