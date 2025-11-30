# DtoAerospikeCluster

AerospikeCluster represents the configuration for an Aerospike cluster for backup.

## Properties

| Name                   | Type                                       |
| ---------------------- | ------------------------------------------ |
| `connTimeout`          | number                                     |
| `credentials`          | [DtoCredentials](DtoCredentials.md)        |
| `label`                | string                                     |
| `maxParallelScans`     | number                                     |
| `preferRacks`          | Array&lt;number&gt;                        |
| `seedNodes`            | [Array&lt;DtoSeedNode&gt;](DtoSeedNode.md) |
| `tls`                  | [DtoTLS](DtoTLS.md)                        |
| `useServicesAlternate` | boolean                                    |

## Example

```typescript
import type { DtoAerospikeCluster } from "";

// TODO: Update the object below with actual values
const example = {
  connTimeout: 5000,
  credentials: null,
  label: testCluster,
  maxParallelScans: 100,
  preferRacks: null,
  seedNodes: null,
  tls: null,
  useServicesAlternate: null,
} satisfies DtoAerospikeCluster;

console.log(example);

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example);
console.log(exampleJSON);

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoAerospikeCluster;
console.log(exampleParsed);
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
