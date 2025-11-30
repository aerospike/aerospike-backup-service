# DtoTLS

TLS represents the Aerospike cluster TLS configuration options.

## Properties

| Name              | Type   |
| ----------------- | ------ |
| `caFile`          | string |
| `caPath`          | string |
| `certFile`        | string |
| `cipherSuite`     | string |
| `keyFile`         | string |
| `keyFilePassword` | string |
| `name`            | string |
| `protocols`       | string |

## Example

```typescript
import type { DtoTLS } from ''

// TODO: Update the object below with actual values
const example = {
  "caFile": /path/to/cafile.pem,
  "caPath": /path/to/ca,
  "certFile": /path/to/certfile.pem,
  "cipherSuite": TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
  "keyFile": /path/to/keyfile.pem,
  "keyFilePassword": file:/path/to/password,
  "name": tls-name,
  "protocols": null,
} satisfies DtoTLS

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoTLS
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
