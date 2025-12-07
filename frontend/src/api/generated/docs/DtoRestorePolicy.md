
# DtoRestorePolicy

RestorePolicy represents a policy for the restore operation.

## Properties

Name | Type
------------ | -------------
`bandwidth` | number
`batchSize` | number
`binList` | Array&lt;string&gt;
`compression` | [DtoCompressionPolicy](DtoCompressionPolicy.md)
`disableBatchWrites` | boolean
`encryption` | [DtoEncryptionPolicy](DtoEncryptionPolicy.md)
`extraTtl` | number
`maxAsyncBatches` | number
`namespace` | [DtoRestoreNamespace](DtoRestoreNamespace.md)
`noGeneration` | boolean
`noIndexes` | boolean
`noRecords` | boolean
`noUdfs` | boolean
`parallel` | number
`replace` | boolean
`retryPolicy` | [DtoRetryPolicy](DtoRetryPolicy.md)
`setList` | Array&lt;string&gt;
`socketTimeout` | number
`totalTimeout` | number
`tps` | number
`unique` | boolean

## Example

```typescript
import type { DtoRestorePolicy } from ''

// TODO: Update the object below with actual values
const example = {
  "bandwidth": 50000,
  "batchSize": 32,
  "binList": ["bin1","bin2"],
  "compression": null,
  "disableBatchWrites": null,
  "encryption": null,
  "extraTtl": 86400,
  "maxAsyncBatches": 32,
  "namespace": null,
  "noGeneration": null,
  "noIndexes": null,
  "noRecords": null,
  "noUdfs": null,
  "parallel": 8,
  "replace": null,
  "retryPolicy": null,
  "setList": ["set1","set2"],
  "socketTimeout": 1000,
  "totalTimeout": 2000,
  "tps": 4000,
  "unique": null,
} satisfies DtoRestorePolicy

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoRestorePolicy
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


