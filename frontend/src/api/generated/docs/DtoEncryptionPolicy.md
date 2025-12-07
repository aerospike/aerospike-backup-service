
# DtoEncryptionPolicy

EncryptionPolicy contains backup encryption information.

## Properties

Name | Type
------------ | -------------
`keyEnv` | string
`keyFile` | string
`keySecret` | string
`mode` | string

## Example

```typescript
import type { DtoEncryptionPolicy } from ''

// TODO: Update the object below with actual values
const example = {
  "keyEnv": null,
  "keyFile": null,
  "keySecret": null,
  "mode": null,
} satisfies DtoEncryptionPolicy

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoEncryptionPolicy
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


