# DtoCredentials

Credentials represents authentication details to the Aerospike cluster.

## Properties

| Name              | Type                                |
| ----------------- | ----------------------------------- |
| `authMode`        | string                              |
| `password`        | string                              |
| `passwordPath`    | string                              |
| `secretAgent`     | [DtoSecretAgent](DtoSecretAgent.md) |
| `secretAgentName` | string                              |
| `user`            | string                              |

## Example

```typescript
import type { DtoCredentials } from "";

// TODO: Update the object below with actual values
const example = {
  authMode: null,
  password: testPswd,
  passwordPath: /path/ot / pass.txt,
  secretAgent: null,
  secretAgentName: null,
  user: testUser,
} satisfies DtoCredentials;

console.log(example);

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example);
console.log(exampleJSON);

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoCredentials;
console.log(exampleParsed);
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
