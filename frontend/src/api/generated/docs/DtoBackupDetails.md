
# DtoBackupDetails

BackupDetails contains information about a finished backup.

## Properties

 Name                  | Type                        
-----------------------|-----------------------------
 `byteCount`           | number                      
 `compression`         | string                      
 `created`             | string                      
 `duration`            | number                      
 `encryption`          | string                      
 `fileCount`           | number                      
 `finished`            | string                      
 `from`                | string                      
 `key`                 | string                      
 `namespace`           | string                      
 `recordCount`         | number                      
 `secondaryIndexCount` | number                      
 `storage`             | [DtoStorage](DtoStorage.md) 
 `timestamp`           | number                      
 `udfCount`            | number                      

## Example

```typescript
import type { DtoBackupDetails } from ''

// TODO: Update the object below with actual values
const example = {
  "byteCount": 2000,
  "compression": null,
  "created": 2023-03-20T14:50:00Z,
  "duration": null,
  "encryption": null,
  "fileCount": 1,
  "finished": 2023-03-20T14:50:00Z,
  "from": 2023-03-19T14:50:00Z,
  "key": daily/backup/1707915600000/source-ns1,
  "namespace": testNamespace,
  "recordCount": 100,
  "secondaryIndexCount": 5,
  "storage": null,
  "timestamp": 1685458200000,
  "udfCount": 2,
} satisfies DtoBackupDetails

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoBackupDetails
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


