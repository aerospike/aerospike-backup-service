# DtoRateLimiterConfig

RateLimiterConfig is the HTTP server rate limiter configuration.

## Properties

| Name        | Type                |
| ----------- | ------------------- |
| `size`      | number              |
| `tps`       | number              |
| `whiteList` | Array&lt;string&gt; |

## Example

```typescript
import type { DtoRateLimiterConfig } from "";

// TODO: Update the object below with actual values
const example = {
  size: 1024,
  tps: 1024,
  whiteList: null,
} satisfies DtoRateLimiterConfig;

console.log(example);

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example);
console.log(exampleJSON);

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoRateLimiterConfig;
console.log(exampleParsed);
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
