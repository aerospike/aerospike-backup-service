
# DtoAzureStorageClass

AzureStorageClass represents the configuration for Azure Blob Storage access tiers.

## Properties

 Name       | Type   
------------|--------
 `data`     | string 
 `metadata` | string 

## Example

```typescript
import type { DtoAzureStorageClass } from ''

// TODO: Update the object below with actual values
const example = {
  "data": null,
  "metadata": null,
} satisfies DtoAzureStorageClass

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoAzureStorageClass
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


