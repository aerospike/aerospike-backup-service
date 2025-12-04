
# DtoAzureStorage

AzureStorage represents the configuration for Azure Blob storage.

## Properties

 Name              | Type                                            
-------------------|-------------------------------------------------
 `accountKey`      | string                                          
 `accountName`     | string                                          
 `clientId`        | string                                          
 `clientSecret`    | string                                          
 `containerName`   | string                                          
 `endpoint`        | string                                          
 `minPartSize`     | number                                          
 `path`            | string                                          
 `secretAgent`     | [DtoSecretAgent](DtoSecretAgent.md)             
 `secretAgentName` | string                                          
 `storageClass`    | [DtoAzureStorageClass](DtoAzureStorageClass.md) 
 `tenantId`        | string                                          

## Example

```typescript
import type { DtoAzureStorage } from ''

// TODO: Update the object below with actual values
const example = {
  "accountKey": null,
  "accountName": null,
  "clientId": null,
  "clientSecret": null,
  "containerName": null,
  "endpoint": null,
  "minPartSize": null,
  "path": backups,
  "secretAgent": null,
  "secretAgentName": null,
  "storageClass": null,
  "tenantId": null,
} satisfies DtoAzureStorage

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoAzureStorage
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


