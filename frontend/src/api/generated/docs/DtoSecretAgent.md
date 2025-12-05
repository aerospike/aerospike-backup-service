
# DtoSecretAgent

SecretAgent represents the configuration of an Aerospike Secret Agent.

## Properties

Name | Type
------------ | -------------
`address` | string
`connectionType` | string
`isBase64` | boolean
`port` | number
`timeout` | number
`tlsCaFile` | string

## Example

```typescript
import type { DtoSecretAgent } from ''

// TODO: Update the object below with actual values
const example = {
  "address": localhost,
  "connectionType": tcp,
  "isBase64": null,
  "port": 8080,
  "timeout": null,
  "tlsCaFile": /path/to/ca.pem,
} satisfies DtoSecretAgent

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoSecretAgent
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


