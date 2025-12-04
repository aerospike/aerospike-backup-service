
# DtoFileLoggerConfig

FileLoggerConfig represents the configuration for the file logger writer.

## Properties

 Name         | Type    
--------------|---------
 `compress`   | boolean 
 `filename`   | string  
 `maxage`     | number  
 `maxbackups` | number  
 `maxsize`    | number  

## Example

```typescript
import type { DtoFileLoggerConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "compress": null,
  "filename": log.txt,
  "maxage": null,
  "maxbackups": null,
  "maxsize": 100,
} satisfies DtoFileLoggerConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoFileLoggerConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


