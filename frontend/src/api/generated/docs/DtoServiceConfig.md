
# DtoServiceConfig

ServiceConfig represents the backup service configuration properties.

## Properties

 Name     | Type                                              
----------|---------------------------------------------------
 `backup` | [DtoBackupCommonConfig](DtoBackupCommonConfig.md) 
 `http`   | [DtoHTTPServerConfig](DtoHTTPServerConfig.md)     
 `logger` | [DtoLoggerConfig](DtoLoggerConfig.md)             

## Example

```typescript
import type { DtoServiceConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "backup": null,
  "http": null,
  "logger": null,
} satisfies DtoServiceConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoServiceConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


