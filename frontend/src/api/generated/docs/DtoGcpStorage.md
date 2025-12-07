
# DtoGcpStorage

GcpStorage represents the configuration for GCP storage.

## Properties

Name | Type
------------ | -------------
`bucketName` | string
`endpoint` | string
`key` | string
`keyFilePath` | string
`minPartSize` | number
`path` | string
`secretAgent` | [DtoSecretAgent](DtoSecretAgent.md)
`secretAgentName` | string
`storageClass` | [DtoGcpStorageClass](DtoGcpStorageClass.md)

## Example

```typescript
import type { DtoGcpStorage } from ''

// TODO: Update the object below with actual values
const example = {
  "bucketName": null,
  "endpoint": null,
  "key": null,
  "keyFilePath": null,
  "minPartSize": null,
  "path": backups,
  "secretAgent": null,
  "secretAgentName": null,
  "storageClass": null,
} satisfies DtoGcpStorage

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoGcpStorage
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


