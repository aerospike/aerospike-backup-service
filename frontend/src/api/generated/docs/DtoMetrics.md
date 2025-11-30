# DtoMetrics

Metrics represents the current job speed.

## Properties

| Name                 | Type   |
| -------------------- | ------ |
| `kilobytesPerSecond` | number |
| `pipeline`           | number |
| `recordsPerSecond`   | number |

## Example

```typescript
import type { DtoMetrics } from "";

// TODO: Update the object below with actual values
const example = {
  kilobytesPerSecond: null,
  pipeline: null,
  recordsPerSecond: null,
} satisfies DtoMetrics;

console.log(example);

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example);
console.log(exampleJSON);

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoMetrics;
console.log(exampleParsed);
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
