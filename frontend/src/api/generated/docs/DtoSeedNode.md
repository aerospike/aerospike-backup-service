
# DtoSeedNode

SeedNode represents details of a node in the Aerospike cluster.

## Properties

 Name       | Type   
------------|--------
 `hostName` | string 
 `port`     | number 
 `tlsName`  | string 

## Example

```typescript
import type { DtoSeedNode } from ''

// TODO: Update the object below with actual values
const example = {
  "hostName": localhost,
  "port": 3000,
  "tlsName": certName,
} satisfies DtoSeedNode

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoSeedNode
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


