
# DtoRestoreTimestampRequest

RestoreTimestampRequest represents a restore by timestamp operation request.

## Properties

Name | Type
------------ | -------------
`destination` | [DtoAerospikeCluster](DtoAerospikeCluster.md)
`destinationName` | string
`disableReordering` | boolean
`policy` | [DtoRestorePolicy](DtoRestorePolicy.md)
`routine` | string
`secretAgent` | [DtoSecretAgent](DtoSecretAgent.md)
`secretAgentName` | string
`time` | number

## Example

```typescript
import type { DtoRestoreTimestampRequest } from ''

// TODO: Update the object below with actual values
const example = {
  "destination": null,
  "destinationName": null,
  "disableReordering": null,
  "policy": null,
  "routine": daily,
  "secretAgent": null,
  "secretAgentName": null,
  "time": 1739538000000,
} satisfies DtoRestoreTimestampRequest

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoRestoreTimestampRequest
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


