
# DtoHTTPServerConfig

HTTPServerConfig represents the service\'s HTTP server configuration.

## Properties

 Name          | Type                                            
---------------|-------------------------------------------------
 `address`     | string                                          
 `contextPath` | string                                          
 `port`        | number                                          
 `rate`        | [DtoRateLimiterConfig](DtoRateLimiterConfig.md) 
 `timeout`     | number                                          

## Example

```typescript
import type { DtoHTTPServerConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "address": 0.0.0.0,
  "contextPath": null,
  "port": 8080,
  "rate": null,
  "timeout": null,
} satisfies DtoHTTPServerConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoHTTPServerConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


