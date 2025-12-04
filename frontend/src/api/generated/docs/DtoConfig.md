
# DtoConfig

Config represents the service configuration file.

## Properties

 Name                | Type                                                              
---------------------|-------------------------------------------------------------------
 `aerospikeClusters` | [{ [key: string]: DtoAerospikeCluster; }](DtoAerospikeCluster.md) 
 `backupPolicies`    | [{ [key: string]: DtoBackupPolicy; }](DtoBackupPolicy.md)         
 `backupRoutines`    | [{ [key: string]: DtoBackupRoutine; }](DtoBackupRoutine.md)       
 `secretAgents`      | [{ [key: string]: DtoSecretAgent; }](DtoSecretAgent.md)           
 `service`           | [DtoServiceConfig](DtoServiceConfig.md)                           
 `storage`           | [{ [key: string]: DtoStorage; }](DtoStorage.md)                   

## Example

```typescript
import type { DtoConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "aerospikeClusters": null,
  "backupPolicies": null,
  "backupRoutines": null,
  "secretAgents": null,
  "service": null,
  "storage": null,
} satisfies DtoConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


