
# DtoRestoreNamespace

RestoreNamespace specifies an alternative namespace name for the restore operation.

## Properties

 Name          | Type   
---------------|--------
 `destination` | string 
 `source`      | string 

## Example

```typescript
import type { DtoRestoreNamespace } from ''

// TODO: Update the object below with actual values
const example = {
  "destination": destination-ns,
  "source": source-ns,
} satisfies DtoRestoreNamespace

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoRestoreNamespace
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


