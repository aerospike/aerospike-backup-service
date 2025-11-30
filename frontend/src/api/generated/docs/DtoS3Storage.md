# DtoS3Storage

S3Storage represents the configuration for S3 storage.

## Properties

| Name                  | Type                                      |
| --------------------- | ----------------------------------------- |
| `accessKeyId`         | string                                    |
| `bucket`              | string                                    |
| `maxAsyncConnections` | number                                    |
| `minPartSize`         | number                                    |
| `path`                | string                                    |
| `s3EndpointOverride`  | string                                    |
| `s3LogLevel`          | string                                    |
| `s3Profile`           | string                                    |
| `s3Region`            | string                                    |
| `secretAccessKey`     | string                                    |
| `secretAgent`         | [DtoSecretAgent](DtoSecretAgent.md)       |
| `secretAgentName`     | string                                    |
| `storageClass`        | [DtoS3StorageClass](DtoS3StorageClass.md) |

## Example

```typescript
import type { DtoS3Storage } from ''

// TODO: Update the object below with actual values
const example = {
  "accessKeyId": null,
  "bucket": null,
  "maxAsyncConnections": 16,
  "minPartSize": null,
  "path": backups,
  "s3EndpointOverride": http://host.docker.internal:9000,
  "s3LogLevel": null,
  "s3Profile": default,
  "s3Region": eu-central-1,
  "secretAccessKey": null,
  "secretAgent": null,
  "secretAgentName": null,
  "storageClass": null,
} satisfies DtoS3Storage

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoS3Storage
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
