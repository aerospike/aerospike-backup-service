
# DtoStorage

Storage represents the configuration for a backup storage details.

## Properties

 Name           | Type                                  
----------------|---------------------------------------
 `azureStorage` | [DtoAzureStorage](DtoAzureStorage.md) 
 `gcpStorage`   | [DtoGcpStorage](DtoGcpStorage.md)     
 `localStorage` | [DtoLocalStorage](DtoLocalStorage.md) 
 `s3Storage`    | [DtoS3Storage](DtoS3Storage.md)       

## Example

```typescript
import type { DtoStorage } from ''

// TODO: Update the object below with actual values
const example = {
  "azureStorage": null,
  "gcpStorage": null,
  "localStorage": null,
  "s3Storage": null,
} satisfies DtoStorage

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoStorage
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


