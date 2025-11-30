# DtoRestoreRequest

RestoreRequest represents a restore operation request.

## Properties

| Name              | Type                                          |
| ----------------- | --------------------------------------------- |
| `backupDataPath`  | string                                        |
| `destination`     | [DtoAerospikeCluster](DtoAerospikeCluster.md) |
| `destinationName` | string                                        |
| `policy`          | [DtoRestorePolicy](DtoRestorePolicy.md)       |
| `secretAgent`     | [DtoSecretAgent](DtoSecretAgent.md)           |
| `secretAgentName` | string                                        |
| `source`          | [DtoStorage](DtoStorage.md)                   |
| `sourceName`      | string                                        |

## Example

```typescript
import type { DtoRestoreRequest } from "";

// TODO: Update the object below with actual values
const example = {
  backupDataPath: null,
  destination: null,
  destinationName: null,
  policy: null,
  secretAgent: null,
  secretAgentName: null,
  source: null,
  sourceName: null,
} satisfies DtoRestoreRequest;

console.log(example);

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example);
console.log(exampleJSON);

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoRestoreRequest;
console.log(exampleParsed);
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
