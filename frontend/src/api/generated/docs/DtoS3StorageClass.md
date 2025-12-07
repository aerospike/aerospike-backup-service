
# DtoS3StorageClass

S3StorageClass represents the configuration for S3 Storage Class.

## Properties

Name | Type
------------ | -------------
`data` | string
`metadata` | string

## Example

```typescript
import type { DtoS3StorageClass } from ''

// TODO: Update the object below with actual values
const example = {
  "data": null,
  "metadata": null,
} satisfies DtoS3StorageClass

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoS3StorageClass
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


