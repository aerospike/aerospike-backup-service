
# DtoBackupPolicy

BackupPolicy represents a scheduled backup policy.

## Properties

Name | Type
------------ | -------------
`bandwidth` | number
`compression` | [DtoCompressionPolicy](DtoCompressionPolicy.md)
`concurrentIncremental` | boolean
`encryption` | [DtoEncryptionPolicy](DtoEncryptionPolicy.md)
`fileLimit` | number
`maxConcurrentNodes` | number
`noIndexes` | boolean
`noRecords` | boolean
`noUdfs` | boolean
`parallel` | number
`parallelWrite` | number
`recordsPerSecond` | number
`retention` | [DtoRetentionPolicy](DtoRetentionPolicy.md)
`retryPolicy` | [DtoRetryPolicy](DtoRetryPolicy.md)
`sealed` | boolean
`socketTimeout` | number
`totalTimeout` | number
`useScanCompression` | boolean
`withClusterConfiguration` | boolean

## Example

```typescript
import type { DtoBackupPolicy } from ''

// TODO: Update the object below with actual values
const example = {
  "bandwidth": 10000,
  "compression": null,
  "concurrentIncremental": null,
  "encryption": null,
  "fileLimit": null,
  "maxConcurrentNodes": null,
  "noIndexes": null,
  "noRecords": null,
  "noUdfs": null,
  "parallel": 1,
  "parallelWrite": 1,
  "recordsPerSecond": 1000,
  "retention": null,
  "retryPolicy": null,
  "sealed": null,
  "socketTimeout": null,
  "totalTimeout": null,
  "useScanCompression": null,
  "withClusterConfiguration": null,
} satisfies DtoBackupPolicy

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoBackupPolicy
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


